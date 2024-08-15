package protogen

import (
	"bytes"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type Plugin interface {
	Generate(gen *Generator, file *File) error

	SupportedFeatures() uint64

	SupportedEditionsMinimum() descriptorpb.Edition

	SupportedEditionsMaximum() descriptorpb.Edition
}

type Generator struct {
	request *pluginpb.CodeGeneratorRequest
	plugin  Plugin

	files          []*File
	fileReg        *protoregistry.Files
	filesByPath    map[string]*File
	enumsByName    map[protoreflect.FullName]*Enum
	messagesByName map[protoreflect.FullName]*Message

	genFiles []*GeneratedFile
	err      error
}

func NewGenerator(req *pluginpb.CodeGeneratorRequest, plugin Plugin) (*Generator, error) {
	gen := &Generator{
		request:        req,
		plugin:         plugin,
		fileReg:        new(protoregistry.Files),
		filesByPath:    make(map[string]*File),
		enumsByName:    make(map[protoreflect.FullName]*Enum),
		messagesByName: make(map[protoreflect.FullName]*Message),
	}

	for _, protoFile := range gen.request.ProtoFile {
		filename := protoFile.GetName()
		if gen.filesByPath[filename] != nil {
			return nil, fmt.Errorf("duplicate file name: %q", filename)
		}

		f, err := newFile(gen, protoFile)
		if err != nil {
			return nil, err
		}

		gen.files = append(gen.files, f)
		gen.filesByPath[filename] = f
	}

	for _, filename := range gen.request.FileToGenerate {
		f, ok := gen.filesByPath[filename]
		if !ok {
			return nil, fmt.Errorf("no descriptor for generated file: %v", filename)
		}
		f.Generate = true
	}

	return gen, nil
}

func (gen *Generator) GenerateFiles() {
	for _, file := range gen.files {
		err := gen.plugin.Generate(gen, file)
		if err != nil {
			gen.err = err
			return
		}
	}
}

func (gen *Generator) ProtocVersion() string {
	v := gen.request.GetCompilerVersion()
	if v == nil {
		return "(unknown)"
	}

	var suffix string
	if s := v.GetSuffix(); s != "" {
		suffix = "-" + s
	}

	return fmt.Sprintf("v%d.%d.%d%s", v.GetMajor(), v.GetMinor(), v.GetPatch(), suffix)
}

func (gen *Generator) Response() *pluginpb.CodeGeneratorResponse {
	resp := &pluginpb.CodeGeneratorResponse{}
	if gen.err != nil {
		resp.Error = proto.String(gen.err.Error())
		return resp
	}

	for _, g := range gen.genFiles {
		if g.skip {
			continue
		}

		content, err := g.Content()
		if err != nil {
			return &pluginpb.CodeGeneratorResponse{
				Error: proto.String(err.Error()),
			}
		}

		filename := g.filename
		resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String(filename),
			Content: proto.String(string(content)),
		})
	}

	p := gen.plugin

	supportedFeatures := p.SupportedFeatures()
	if supportedFeatures > 0 {
		resp.SupportedFeatures = proto.Uint64(supportedFeatures)
	}

	supportedEditionsMinimum := p.SupportedEditionsMinimum()
	supportedEditionsMaximum := p.SupportedEditionsMaximum()
	if supportedEditionsMinimum != descriptorpb.Edition_EDITION_UNKNOWN && supportedEditionsMaximum != descriptorpb.Edition_EDITION_UNKNOWN {
		resp.MinimumEdition = proto.Int32(int32(supportedEditionsMinimum))
		resp.MaximumEdition = proto.Int32(int32(supportedEditionsMaximum))
	}
	return resp
}

type GeneratedFile struct {
	gen      *Generator
	skip     bool
	filename string
	buf      bytes.Buffer
}

func (gen *Generator) NewGeneratedFile(filename string) *GeneratedFile {
	g := &GeneratedFile{
		gen:      gen,
		filename: filename,
	}

	gen.genFiles = append(gen.genFiles, g)
	return g
}

func (g *GeneratedFile) Skip() {
	g.skip = true
}

func (g *GeneratedFile) Unskip() {
	g.skip = false
}

func (g *GeneratedFile) P(v ...any) {
	for _, x := range v {
		fmt.Fprint(&g.buf, x)
	}
	fmt.Fprintln(&g.buf)
}

func (g *GeneratedFile) Write(p []byte) (n int, err error) {
	return g.buf.Write(p)
}

func (g *GeneratedFile) Content() ([]byte, error) {
	return g.buf.Bytes(), nil
}
