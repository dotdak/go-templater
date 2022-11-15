package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dotdak/go-templater/pkg/module"
	"github.com/dotdak/go-templater/pkg/shorten"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/yoheimuta/go-protoparser/v4"
	proto_parser "github.com/yoheimuta/go-protoparser/v4/parser"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

var (
	ExitFailure = errors.New("exit failure")
	ErrOddParam = errors.New("missing params or values")
	ErrNoInput  = errors.New("no input file")

	genCmd = &ffcli.Command{
		Name:       "gen",
		ShortUsage: "gotem gen [commands flags]",
		ShortHelp:  "Generate template files",
		FlagSet: func() *flag.FlagSet {
			fs := newFlagSet("gen")
			fs.StringVar(&genArgs.in, "in", "", "input package directory")
			fs.StringVar(&genArgs.out, "out", "./handlers/v1", "output directory")
			fs.StringVar(&genArgs.domain, "domain", "Handler", "specify generated domain")
			fs.StringVar(&genArgs.subDomain, "subdomain", "Service", "specify generated domain")
			fs.StringVar(&genArgs.subDomainOut, "subdomain-out", "./services", "specify generated domain")
			fs.BoolVar(&genArgs.overWrite, "overwrite", true, "overwrite existed generated files")
			return fs
		}(),
		Exec: generate,
	}
	goPath     = os.Getenv("HOME") + "/go"
	versionReg = regexp.MustCompile("@v[0-9.]+-[0-9a-z]+-[0-9a-z]+")

	genArgs struct {
		in           string
		out          string
		subDomainOut string
		domain       string
		subDomain    string
		overWrite    bool
		// WIP
		inProto string
	}
)

func load(ctx context.Context, args ...string) ([]*packages.Package, []error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, []error{err}
	}
	cfg := &packages.Config{
		Context: ctx,
		Mode:    packages.LoadAllSyntax,
		Dir:     wd,
		Env:     os.Environ(),
	}

	escaped := make([]string, len(args))
	for i := range args {
		escaped[i] = "pattern=" + args[i]
	}
	pkgs, err := packages.Load(cfg, args...)
	if err != nil {
		return nil, []error{err}
	}

	var errs []error
	for _, p := range pkgs {
		for _, e := range p.Errors {
			errs = append(errs, e)
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return pkgs, nil
}

func logErrors(errs ...error) {
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "%s", e.Error())
	}
}

// WIP
func readProto() error {
	absPath, err := filepath.Abs(genArgs.inProto)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(absPath)
	if err != nil {
		return err
	}

	generator := DomainGenerator{}

	if parts := strings.Split(genArgs.out, "/"); len(parts) > 0 {
		generator.Package = parts[len(parts)-1]
	}
	// if parts := strings.Split(genArgs.in, "/"); len(parts) > 0 {
	// 	generator.FileName = parts[len(parts)-1]
	// }

	proto, err := protoparser.Parse(inputFile)
	if err != nil {
		return err
	}

	for _, body := range proto.ProtoBody {
		if pkg, ok := body.(*proto_parser.Option); ok && pkg.OptionName == "go_package" {
			goPackage := strings.Trim(pkg.Constant, "\"")
			if parts := strings.Split(goPackage, ";"); len(parts) > 0 {
				generator.ImportPackage = parts[0]
			}
			continue
		}
		service, ok := body.(*proto_parser.Service)
		if !ok {
			continue
		}

		for _, serviceBody := range service.ServiceBody {
			rpc, ok := serviceBody.(*proto_parser.RPC)
			if !ok {
				continue
			}

			fmt.Println(rpc.RPCName)
		}
	}
	return nil
}

func getPackageFromDir(dir string) string {
	parts := strings.Split(dir, "/")
	return parts[len(parts)-1]
}

func generate(ctx context.Context, args []string) error {
	inAbs, err := filepath.Abs(genArgs.in)
	if err != nil {
		return err
	}
	outAbs, err := filepath.Abs(genArgs.out)
	if err != nil {
		return err
	}
	subOutAbs, err := filepath.Abs(genArgs.subDomainOut)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, inAbs, func(fi fs.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), "_grpc.pb.go")
	}, parser.ParseComments)
	if err != nil {
		return err
	}
	const preAlloc = 5
	domainFiles := make([]*DomainGenerator, 0, preAlloc)
	intFiles := make(map[string]*IntGen)
	for pkgName, v := range packages {
		for fileName, fi := range v.Files {
			pkgPath := inAbs
			if strings.HasPrefix(inAbs, goPath+"/pkg/mod") {
				pkgPath = strings.TrimPrefix(pkgPath, goPath+"/pkg/mod/")
			} else {
				pkgPath = "github.com" + inAbs
			}

			pkgPath = versionReg.ReplaceAllString(pkgPath, "")
			pkgPath, err = module.DecodePath(pkgPath)
			if err != nil {
				ErrLog.Println(err)
				continue
			}
			parts := strings.Split(fileName, "/")
			domainFile := &DomainGenerator{
				FileName: fmt.Sprintf(
					"%s/%s_%s.go",
					outAbs,
					shorten.TrimFileName(parts[len(parts)-1]),
					strings.ToLower(genArgs.domain),
				),
				Package: getPackageFromDir(outAbs),
				Imports: []*Import{
					{Path: "context"},
					{Path: pkgPath},
					{Name: shorten.Lookup(genArgs.subDomain), Path: "github.com/" + subOutAbs[strings.Index(subOutAbs, "go-templater"):]},
				},
				ServicePackage: pkgName,
				Domain:         genArgs.domain,
			}
			intFile := &IntGen{
				FileName: fmt.Sprintf(
					"%s/%s_%s.go",
					subOutAbs,
					shorten.TrimFileName(parts[len(parts)-1]),
					strings.ToLower(genArgs.subDomain),
				),
				Package: getPackageFromDir(subOutAbs),
				Domain:  genArgs.subDomain,
				Imports: []*Import{
					{Path: "context"},
					{Path: pkgPath},
				},
			}
			astutil.Apply(fi, nil, func(c *astutil.Cursor) bool {
				switch x := c.Node().(type) {
				case *ast.TypeSpec:
					y, ok := x.Type.(*ast.InterfaceType)
					if !ok {
						return true
					}
					intName := x.Name.Name
					if !strings.HasSuffix(intName, "Server") ||
						strings.HasPrefix(intName, "Unimplemented") ||
						strings.HasPrefix(intName, "Unsafe") {
						return true
					}
					intName = strings.TrimSuffix(intName, "ServiceServer")
					subDomainName := intName + genArgs.subDomain
					domainBody := &DomainBody{
						ServiceName: intName,
						Injectors: []*Injector{{
							Name:    subDomainName,
							Alias:   shorten.LowerFirst(subDomainName),
							Package: shorten.Lookup(genArgs.subDomain),
						}},
					}
					intBody := &IntBody{
						Name: intName,
					}

					for _, metRaw := range y.Methods.List {
						metName := metRaw.Names[0].Name
						if strings.HasPrefix(metName, "mustEmbedUnimplemented") {
							continue
						}
						met := metRaw.Type.(*ast.FuncType)
						methodBody := &MethodBody{
							Name: metName,
						}
						for _, paramRaw := range met.Params.List {
							if param, ok := paramRaw.Type.(*ast.SelectorExpr); ok {
								paramType := fmt.Sprintf("%v.%v", param.X, param.Sel.Name)
								paramName := shorten.Lookup(param.Sel.Name)
								methodBody.Args = append(methodBody.Args, &Args{
									Alias: paramName,
									Type:  paramType,
								})
								continue
							}
							if param, ok := paramRaw.Type.(*ast.StarExpr); ok {
								paramType := fmt.Sprint(param.X)
								paramName := shorten.Lookup(paramType)
								paramType = fmt.Sprintf("*%s.%s", pkgName, paramType)
								methodBody.Args = append(methodBody.Args, &Args{
									Alias: paramName,
									Type:  paramType,
								})
								continue
							}
						}
						for _, retRaw := range met.Results.List {
							if ret, ok := retRaw.Type.(*ast.StarExpr); ok {
								retType := fmt.Sprint(ret.X)
								retType = fmt.Sprintf("*%s.%s", v.Name, retType)
								methodBody.Returns = append(methodBody.Returns, &Args{
									Type: retType,
								})
								continue
							}
							if ret, ok := retRaw.Type.(*ast.Ident); ok {
								methodBody.Returns = append(methodBody.Returns, &Args{
									Type: ret.Name,
								})
								continue
							}
						}
						domainBody.Methods = append(domainBody.Methods, methodBody)
						intBody.Methods = append(intBody.Methods, methodBody)
					}
					domainFile.Body = append(domainFile.Body, domainBody)
					intFile.Body = append(intFile.Body, intBody)
				default:
				}
				return true
			})
			domainFiles = append(domainFiles, domainFile)
			for _, in := range intFile.Body {
				intFiles[in.Name] = intFile
			}
		}
	}

	// entitiesPkgs, err := parser.ParseDir(fset, absPath, func(fi fs.FileInfo) bool {
	// 	return strings.HasSuffix(fi.Name(), ".pb.go") && !strings.HasSuffix(fi.Name(), "_grpc.pb.go")
	// }, parser.ParseComments)
	// if err != nil {
	// 	return err
	// }

	// for _, v := range entitiesPkgs {
	// 	for fileName, fi := range v.Files {
	// 		astutil.Apply(fi, nil, func(c *astutil.Cursor) bool {
	// 			switch x := c.Node().(type) {
	// 			case *ast.ImportSpec:
	// 				if !strings.Contains(x.Path.Value, "entities") {
	// 					return true
	// 				}

	// 				x.Name
	// 				intGen.Imports = append(intGen.Imports, x.Path.Value)
	// 			}

	// 			return true
	// 		})
	// 		intGens = append(intGens, intGen)
	// 	}
	// }

	if err := os.MkdirAll(outAbs, os.ModePerm); err != nil {
		return err
	}

	if err := os.MkdirAll(subOutAbs, os.ModePerm); err != nil {
		return err
	}

	for _, fi := range domainFiles {
		if err := fi.WriteFile(genArgs.overWrite); err != nil {
			ErrLog.Println(err)
		}
	}

	for _, fi := range intFiles {
		if err := fi.WriteFile(genArgs.overWrite); err != nil {
			ErrLog.Println(err)
		}
	}

	return nil
}
