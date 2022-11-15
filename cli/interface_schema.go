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

//go:embed sample/interface
var interfaceSample string

type IntGen struct {
	FileName string
	Package  string
	Domain   string
	Imports  []*Import
	Body     []*IntBody
}

type IntBody struct {
	Name    string
	Comment string
	Methods []*MethodBody
}

func (g *IntGen) Render() ([]byte, error) {
	tmpl, err := template.New("tmp").Parse(interfaceSample)
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

func (g *IntGen) WriteFile(overwrite bool) error {
	if _, err := os.Stat(g.FileName); !errors.Is(err, os.ErrNotExist) && !overwrite {
		WarnLog.Printf("ignore %s, file exists", g.FileName)
		return nil
	}
	src, err := g.Render()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(g.FileName, src, os.ModePerm)
}

func (g *IntGen) Print(args ...any) error {
	src, err := g.Render()
	if err != nil {
		ErrLog.Println(err)
		return err
	}

	fmt.Println(string(src))
	return nil
}
