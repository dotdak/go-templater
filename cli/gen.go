package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-templater/pkg/module"
	"github.com/go-templater/pkg/shorten"

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
			fs.StringVar(&genArgs.out, "out", "handlers/v1", "output directory")
			fs.StringVar(&genArgs.domain, "domain", "Handler", "specify generated domain")
			fs.StringVar(&genArgs.subDomain, "subdomain", "Service", "specify generated domain")
			fs.StringVar(&genArgs.subDomainOut, "subdomain out", "services", "specify generated domain")
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

func generate(ctx context.Context, args []string) error {
	absPath, err := filepath.Abs(genArgs.in)
	if err != nil {
		return err
	}

	cfg := &packages.Config{
		Context: ctx,
		Mode:    packages.LoadAllSyntax,
		Dir:     absPath,
		Env:     os.Environ(),
	}

	pkgs, err := packages.Load(cfg, absPath)
	if err != nil {
		return err
	}

	for _, pkg := range pkgs {
		for _, goFile := range pkg.GoFiles {
			if !strings.HasSuffix(goFile, "_grpc.pb.go") {
				continue
			}

			_, err := ioutil.ReadFile(goFile)
			if err != nil {
				fmt.Fprint(os.Stderr, err.Error())
				continue
			}

		}
	}
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, absPath, func(fi fs.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), "_grpc.pb.go")
	}, parser.ParseComments)
	if err != nil {
		return err
	}
	// newFset := token.NewFileSet()
	for pkgName, v := range packages {
		for fileName, fi := range v.Files {
			pkgPath := absPath
			if strings.HasPrefix(absPath, goPath+"/pkg/mod") {
				pkgPath = strings.TrimPrefix(pkgPath, goPath+"/pkg/mod/")
			} else {
				pkgPath = "github.com" + absPath
			}

			pkgPath = versionReg.ReplaceAllString(pkgPath, "")
			pkgPath, err = module.DecodePath(pkgPath)
			if err != nil {
				ErrLog.Println(err)
				continue
			}

			fileGen := &DomainGenerator{
				Package: "v1",
				Imports: []string{
					"context",
					pkgPath,
				},
				ServicePackage: pkgName,
				Domain:         genArgs.domain,
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
					domainBody := &DomainBody{
						ServiceName: strings.TrimSuffix(intName, "Server"),
					}
					for _, metRaw := range y.Methods.List {
						metName := metRaw.Names[0].Name
						if strings.HasPrefix("mustEmbedUnimplemented", metName) {
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
						// for _, retRaw := range met.Results.List {
						// 	fmt.Printf("%T\n", retRaw.Type)
						// 	if ret, ok := retRaw.Type.(*ast.StarExpr); ok {
						// 		retType := fmt.Sprint(ret.X)
						// 		retName := shorten.Lookup(retType)
						// 		retType = fmt.Sprintf("*%s.%s", v.Name, retType)
						// 		methodBody.Returns = append(methodBody.Returns, &Args{
						// 			Alias: retName, Type: retType,
						// 		})
						// 		continue
						// 	}
						// 	if ret, ok := retRaw.Type.(*ast.Ident); ok {
						// 		methodBody.Returns = append(methodBody.Returns, &Args{
						// 			Alias: shorten.Lookup(ret.Name), Type: ret.Name,
						// 		})
						// 		continue
						// 	}
						// }
						domainBody.Methods = append(domainBody.Methods, methodBody)
					}
					fileGen.Body = append(fileGen.Body, domainBody)

					// buf, _ := json.Marshal(x)
					// fmt.Println("block", string(buf))
				default:
					// fmt.Printf("default %T\n", x)
					// buf, _ := json.Marshal(x)
					// fmt.Println("block", string(buf))
					// fmt.Printf("value %v\n", x)
				}
				return true
			})
			// io, e := os.Create(fmt.Sprintf("%s/%s.go", genArgs.out, name))
			// if e != nil {
			// 	ErrLog.Println(e)
			// 	continue
			// }
			// io.Close()

			// printer.Fprint(io, newFset, newFile)
			// printer.Fprint(os.Stdout, newFset, newFile)
			parts := strings.Split(fileName, "/")
			name := fmt.Sprintf("./%s/%s_handlers.go", genArgs.out, strings.TrimSuffix(parts[len(parts)-1], ".pb.go"))
			fmt.Fprint(io.Discard, name)
			if err := fileGen.WriteFile(name); err != nil {
				ErrLog.Println(err)
			}
		}
		// fmt.Println(k, string(buf))
	}
	// buf, _ := json.Marshal(fset)
	// fmt.Println(string(buf))

	if err := os.MkdirAll(genArgs.out, os.ModePerm); err != nil {
		return err
	}

	// goPath := os.Getenv("HOME") + "/go"
	// fmt.Println(goPath)
	// fset := token.NewFileSet()
	// file, err := fileproto_parser.ParseFile(fset, "~/go/pkg/mod/github.com/codeZdeco/evere-proto/go/v1/services/agency_ctl_service.pb.go", nil, fileproto_parser.ParseComments)
	// if err != nil {
	// 	return err
	// }

	// fmt.Println(file)
	return nil
	// return generator.gen(genArgs.out)
}
