package devstack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-unixfsnode"
	"github.com/ipfs/go-unixfsnode/data"
	"github.com/ipfs/go-unixfsnode/file"
	"github.com/ipld/go-car/v2/blockstore"
	dagpb "github.com/ipld/go-codec-dagpb"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
)

// copied from https://github.com/ipld/go-car/blob/master/cmd/car/extract.go

var ErrNotDir = fmt.Errorf("not a directory")

// ExtractCar pulls files and directories out of a car
func ExtractCar(ctx context.Context, file string, outputDir string) error {
	bs, err := blockstore.OpenReadOnly(file)
	if err != nil {
		return err
	}

	ls := cidlink.DefaultLinkSystem()
	ls.TrustedStorage = true
	ls.StorageReadOpener = func(_ ipld.LinkContext, l ipld.Link) (io.Reader, error) {
		cl, ok := l.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("not a cidlink")
		}
		blk, err := bs.Get(ctx, cl.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(blk.RawData()), nil
	}

	roots, err := bs.Roots()
	if err != nil {
		return err
	}

	for _, root := range roots {
		if err := extractRoot(ctx, &ls, root, outputDir); err != nil {
			return err
		}
	}

	return nil
}

func extractRoot(ctx context.Context, ls *ipld.LinkSystem, root cid.Cid, outputDir string) error {
	if root.Prefix().Codec == cid.Raw {
		return nil
	}

	pbn, err := ls.Load(ipld.LinkContext{}, cidlink.Link{Cid: root}, dagpb.Type.PBNode)
	if err != nil {
		return err
	}
	pbnode := pbn.(dagpb.PBNode)

	ufn, err := unixfsnode.Reify(ipld.LinkContext{}, pbnode, ls)
	if err != nil {
		return err
	}

	outputResolvedDir, err := filepath.EvalSymlinks(outputDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(outputResolvedDir); os.IsNotExist(err) {
		if err := os.Mkdir(outputResolvedDir, 0755); err != nil {
			return err
		}
	}
	if err := extractDir(ctx, ls, ufn, outputResolvedDir, "/"); err != nil {
		if !errors.Is(err, ErrNotDir) {
			return fmt.Errorf("%s: %w", root, err)
		}
		ufsData, err := pbnode.LookupByString("Data")
		if err != nil {
			return err
		}
		ufsBytes, err := ufsData.AsBytes()
		if err != nil {
			return err
		}
		ufsNode, err := data.DecodeUnixFSData(ufsBytes)
		if err != nil {
			return err
		}
		if ufsNode.DataType.Int() == data.Data_File || ufsNode.DataType.Int() == data.Data_Raw {
			if err := extractFile(ctx, ls, pbnode, filepath.Join(outputResolvedDir, "unknown")); err != nil {
				return err
			}
		}
		return nil
	}

	return nil
}

func resolvePath(root, pth string) (string, error) {
	rp, err := filepath.Rel("/", pth)
	if err != nil {
		return "", fmt.Errorf("couldn't check relative-ness of %s: %w", pth, err)
	}
	joined := path.Join(root, rp)

	basename := path.Dir(joined)
	final, err := filepath.EvalSymlinks(basename)
	if err != nil {
		return "", fmt.Errorf("couldn't eval symlinks in %s: %w", basename, err)
	}
	if final != path.Clean(basename) {
		return "", fmt.Errorf("path attempts to redirect through symlinks")
	}
	return joined, nil
}

func extractDir(ctx context.Context, ls *ipld.LinkSystem, n ipld.Node, outputRoot, outputPath string) error {
	dirPath, err := resolvePath(outputRoot, outputPath)
	if err != nil {
		return err
	}
	// make the directory.
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	if n.Kind() == ipld.Kind_Map {
		mi := n.MapIterator()
		for !mi.Done() {
			key, val, err := mi.Next()
			if err != nil {
				return err
			}
			ks, err := key.AsString()
			if err != nil {
				return err
			}
			nextRes, err := resolvePath(outputRoot, path.Join(outputPath, ks))
			if err != nil {
				return err
			}

			if val.Kind() != ipld.Kind_Link {
				return fmt.Errorf("unexpected map value for %s at %s", ks, outputPath)
			}
			// a directory may be represented as a map of name:<link> if unixADL is applied
			vl, err := val.AsLink()
			if err != nil {
				return err
			}
			dest, err := ls.Load(ipld.LinkContext{}, vl, basicnode.Prototype.Any)
			if err != nil {
				return err
			}
			// degenerate files are handled here.
			if dest.Kind() == ipld.Kind_Bytes {
				if err := extractFile(ctx, ls, dest, nextRes); err != nil {
					return err
				}
				continue
			} else {
				// dir / pbnode
				pbb := dagpb.Type.PBNode.NewBuilder()
				if err := pbb.AssignNode(dest); err != nil {
					return err
				}
				dest = pbb.Build()
			}
			pbnode := dest.(dagpb.PBNode)

			// interpret dagpb 'data' as unixfs data and look at type.
			ufsData, err := pbnode.LookupByString("Data")
			if err != nil {
				return err
			}
			ufsBytes, err := ufsData.AsBytes()
			if err != nil {
				return err
			}
			ufsNode, err := data.DecodeUnixFSData(ufsBytes)
			if err != nil {
				return err
			}
			if ufsNode.DataType.Int() == data.Data_Directory || ufsNode.DataType.Int() == data.Data_HAMTShard {
				ufn, err := unixfsnode.Reify(ipld.LinkContext{}, pbnode, ls)
				if err != nil {
					return err
				}

				if err := extractDir(ctx, ls, ufn, outputRoot, path.Join(outputPath, ks)); err != nil {
					return err
				}
			} else if ufsNode.DataType.Int() == data.Data_File || ufsNode.DataType.Int() == data.Data_Raw {
				if err := extractFile(ctx, ls, pbnode, nextRes); err != nil {
					return err
				}
			} else if ufsNode.DataType.Int() == data.Data_Symlink {
				data := ufsNode.Data.Must().Bytes()
				if err := os.Symlink(string(data), nextRes); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return ErrNotDir
}

func extractFile(ctx context.Context, ls *ipld.LinkSystem, n ipld.Node, outputName string) error {
	node, err := file.NewUnixFSFile(ctx, n, ls)
	if err != nil {
		return err
	}
	nlr, err := node.AsLargeBytes()
	if err != nil {
		return err
	}

	f, err := os.Create(outputName)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, nlr)

	return err
}
