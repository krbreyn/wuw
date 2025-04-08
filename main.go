package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Directory struct {
	Name  string
	Files []*FileReader
}

type FileReader struct {
	Name string
	R    *bufio.Reader
}

type Package struct {
	Name string
	Deps []string
}

var usage = func() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, "'wuw' is a program for quickly seeing what parts of your Go project depend on what other parts of your project, or what external dependencies they use, so that you can quickly understand the architecture of a codebase.")

	fmt.Fprintf(w, "Usage: %s [-opts] [dirs...]\nopts:\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage

	// subdirsVar := flag.Bool("subdirs", false, "Include sub-directories/packages.")
	// externalVar := flag.Bool("no-external", false, "Exclude external package dependencies (except for golang.org/x/).")
	noStdVar := flag.Bool("no-std", false, "Exclude stdlib packages (including golang.org/x/")

	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var pkgs []Package
	var errs []error

	for _, d := range args {
		entry, err := os.ReadDir(d)
		if err != nil {
			fmt.Println("error reading directory:", err.Error())
			os.Exit(1)
		}

		go_files := GetGoFiles(d, entry)
		if len(go_files) == 0 {
			errs = append(errs, fmt.Errorf("error: no .go files in %s", d))
			continue
		}

		dir := Directory{Name: d}
		for _, g := range go_files {
			f, err := os.Open(g)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			dir.Files = append(dir.Files, &FileReader{g, bufio.NewReader(f)})
		}

		// get package name
		pkg_name, err := GetPackageName(&dir)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// get imports
		var imports []string
		for _, f := range dir.Files {
			i, err := ParseFileForImports(f.R)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, s := range i {
				if !slices.Contains(imports, s) {
					imports = append(imports, s)
				}
			}
		}

		pkgs = append(pkgs, Package{Name: pkg_name, Deps: imports})
	}

	if len(errs) != 0 {
		fmt.Println("errors:")
		for _, err := range errs {
			fmt.Println(err)
		}
		os.Exit(1)
	} else {
		for _, p := range pkgs {
			for _, d := range FilterDependencies(p.Deps, false, *noStdVar) {
				fmt.Printf("%s %s\n", p.Name, d)
			}
		}
		os.Exit(0)
	}
}

// TODO
func FilterDependencies(deps []string, external, noStd bool) []string {
	var ret []string
	for _, d := range deps {
		if noStd {
			pkg, err := build.Import(d, "", build.FindOnly)
			if strings.Contains(d, "golang.org/x/") || (err == nil && pkg.Goroot) {
				continue
			}
		}
		ret = append(ret, d)
	}
	return ret
}

// TODO properly parse instead of relying on gofmt conventions?
func ParseFileForImports(r *bufio.Reader) ([]string, error) {
	var imports []string

	var linesWithoutImport int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			linesWithoutImport++
			continue
		}
		if strings.Contains(line, "import \"") {
			split := strings.Fields(line)
			imports = append(imports, split[1])
			continue
		} else if strings.Contains(line, "import (") {
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return nil, err
				}
				ts := strings.TrimSpace(line)
				if ts == ")" {
					break
				}
				if ts == "" {
					continue
				}
				split := strings.Fields(line)
				var imp string
				if len(split) == 2 {
					imp = split[1]
				} else {
					imp = split[0]
				}
				imports = append(imports, imp[1:len(imp)-1])
			}
		} else {
			linesWithoutImport++
		}

		if linesWithoutImport >= 5 {
			break
		}
	}

	return imports, nil
}

func GetPackageName(d *Directory) (string, error) {
	seen := make(map[string]struct{})
	var pkg_name string

	for _, r := range d.Files {
		line, err := r.R.ReadString('\n')
		if err != nil {
			return "", err
		}

		fields := strings.Fields(line)

		if len(fields) != 2 || fields[0] != "package" {
			return "", fmt.Errorf("error: malformed package line: %s in file %s", line, r.Name)
		}

		pkg_name = fields[1]
		seen[pkg_name] = struct{}{}
	}

	if len(seen) == 0 {
		return "", fmt.Errorf("could not find a package in dir %s", d.Name)
	}

	if len(seen) != 1 {
		return "", fmt.Errorf("more than one package declaration in folder %s", d.Name)
	}

	return pkg_name, nil
}

func GetGoFiles(dir_name string, dir []os.DirEntry) []string {
	var go_files []string

	for _, f := range dir {
		if strings.HasPrefix(f.Name(), ".") {
			continue // hidden file
		}

		if f.IsDir() {
			continue
		}

		n := filepath.Join(dir_name, f.Name())

		if filepath.Ext(n) == ".go" {
			go_files = append(go_files, n)
		}
	}

	return go_files
}

func GetDirectories(dir_name string, dir []os.DirEntry) []string {
	return nil
}

func GatherSubdirs(dir []os.DirEntry) [][]os.DirEntry {
	return nil
}
