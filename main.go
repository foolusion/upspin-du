package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"upspin.io/client"
	"upspin.io/config"
	_ "upspin.io/dir/remote"
	"upspin.io/flags"
	_ "upspin.io/key/transports"
	"upspin.io/subcmd"
	"upspin.io/transports"
	"upspin.io/upspin"
)

type state struct {
	*subcmd.State
	client  *client.Client
	entries []*upspin.DirEntry
}

const help = `
Du tells you the disk usage of an upspin directory that you can access.
`

func main() {
	const name = "du"

	log.SetFlags(0)
	log.SetPrefix("upspin du: ")

	s := &state{
		State: subcmd.NewState(name),
	}

	human := flag.Bool("h", false, "print size in human readable format")
	depth := flag.Int("d", -1, "depth to recur in directories")

	s.ParseFlags(flag.CommandLine, os.Args[1:], help, "du -h -d=<depth> <path>")

	cfg, err := config.FromFile(flags.Config)
	if err != nil && err != config.ErrNoFactotum {
		s.Exit(err)
	}
	transports.Init(cfg)
	s.State.Init(cfg)

	if flag.Arg(0) == "" {
		s.Exitf("must supply a path")
	}

	done := map[upspin.PathName]int64{}
	for _, entry := range s.GlobAllUpspin(flag.Args()) {
		root := &tree{DirEntry: entry}

		s.list(entry, root, done)

		if len(root.children) != 1 {
			s.Exitf("something weird happended")
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
		if *depth >= 0 {
			printDepthEntries(w, root.children[0], done, *human, *depth)
		} else {
			printEntries(w, root.children[0], done, *human)
		}
		w.Flush()
	}
	s.ExitNow()
}

type tree struct {
	*upspin.DirEntry
	children []*tree
}

func (t *tree) String() string {
	return string(t.Name)
}

func (s *state) list(entry *upspin.DirEntry, parent *tree, done map[upspin.PathName]int64) {
	if entry.IsDir() {
		dirNode := &tree{DirEntry: entry}
		parent.children = append(parent.children, dirNode)
		dirContents, err := s.Client.Glob(upspin.AllFilesGlob(entry.Name))
		if err != nil {
			s.Exit(err)
		}
		for _, subEntry := range dirContents {
			if size, ok := done[subEntry.Name]; !ok || size == 0 {
				s.list(subEntry, dirNode, done)
			}
			done[entry.Name] += done[subEntry.Name]
		}
	}
	if entry.IsRegular() || entry.IsLink() {
		fileNode := &tree{DirEntry: entry}
		parent.children = append(parent.children, fileNode)
		done[entry.Name] += size(entry)
	}
}

func size(entry *upspin.DirEntry) (size int64) {
	for _, b := range entry.Blocks {
		size += b.Size
	}
	return
}

func printDepthEntries(w *tabwriter.Writer, node *tree, sizes map[upspin.PathName]int64, human bool, depth int) {
	if depth < 0 {
		return
	}
	for _, child := range node.children {
		printDepthEntries(w, child, sizes, human, depth-1)
	}
	if node.IsDir() && human {
		fmt.Fprintln(w, humanEntry(node, sizes))
	} else if node.IsDir() {
		fmt.Fprintf(w, "%d\t%s\n", sizes[node.Name], node)
	}
}

func printEntries(w *tabwriter.Writer, node *tree, sizes map[upspin.PathName]int64, human bool) {
	for _, child := range node.children {
		printEntries(w, child, sizes, human)
	}
	if human {
		fmt.Fprintln(w, humanEntry(node, sizes))
	} else {
		fmt.Fprintf(w, "%d\t%s\n", sizes[node.Name], node)
	}
}

func humanEntry(node *tree, sizes map[upspin.PathName]int64) string {
	scaleName := []string{"B", "K", "M", "G", "T"}
	fsize := float64(sizes[node.Name])
	scale := 0
	for fsize > 1024 {
		scale++
		fsize /= 1024
	}
	return fmt.Sprintf("%.1f%s\t%s", fsize, scaleName[scale], node)
}
