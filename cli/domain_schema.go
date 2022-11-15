package cli

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"text/template"
)

//go:embed sample/domain
var sample string

type DomainGenerator struct {
	FileName       string
	Imports        []*Import
	ImportPackage  string
	Package        string
	Domain         string
	ServicePackage string
	Body           []*DomainBody
}

type Injector struct {
	Alias   string
	Name    string
	Package string
}

type Import struct {
	Name string
	Path string
}

type DomainBody struct {
	ServiceName string
	Comment     string
	Injectors   []*Injector
	Methods     []*MethodBody
	Args        []*Args
	Returns     []*Args
}

type MethodBody struct {
	Comment string
	Name    string
	Args    []*Args
	Returns []*Args
}

type Args struct {
	Alias string
	Type  string
}

func (g *DomainGenerator) Render() ([]byte, error) {
	tmpl, err := template.New("tmp").Parse(sample)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	if err := tmpl.Execute(&b, g); err != nil {
		return nil, err
	}
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (g *DomainGenerator) Print(args ...any) error {
	src, err := g.Render()
	if err != nil {
		ErrLog.Println(err)
		return err
	}

	fmt.Println(string(src))
	return nil
}

func (g *DomainGenerator) WriteFile(overwrite bool) error {
	if _, err := os.Stat(g.FileName); !errors.Is(err, os.ErrNotExist) && !overwrite {
		WarnLog.Printf("ignore %s, file exists", g.FileName)
		return nil
	}
	src, err := g.Render()
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	return ioutil.WriteFile(g.FileName, src, os.ModePerm)
}
