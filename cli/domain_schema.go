package cli

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"io/fs"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

//go:embed sample/domain
var sample string

type DomainGenerator struct {
	Imports        []string
	ImportPackage  string
	Package        string
	Domain         string
	ServicePackage string
	Body           []*DomainBody
}

type DomainBody struct {
	ServiceName string
	Comment     string
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

func (g *DomainGenerator) gen(filename string) error {
	body := strings.Builder{}
	body.WriteString("package ")
	body.WriteString(g.Package)
	body.WriteRune('\n')
	body.WriteString("OK")
	return ioutil.WriteFile(filename, []byte(body.String()), fs.ModePerm)
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

func (g *DomainGenerator) Print() {
	src, err := g.Render()
	if err != nil {
		ErrLog.Println(err)
		return
	}

	fmt.Println(string(src))
}

func (g *DomainGenerator) WriteFile(filename string) error {
	src, err := g.Render()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, src, os.ModePerm)
}
