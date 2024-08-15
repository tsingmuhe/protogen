package protogen

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// A File describes a .proto source file.
type File struct {
	Proto *descriptorpb.FileDescriptorProto
	Desc  protoreflect.FileDescriptor

	Enums      []*Enum      // top-level enum declarations
	Messages   []*Message   // top-level message declarations
	Extensions []*Extension // top-level extension declarations
	Services   []*Service   // top-level service declarations

	Location Location

	Generate bool // true if we should generate code for this file
}

func newFile(gen *Generator, p *descriptorpb.FileDescriptorProto) (*File, error) {
	desc, err := protodesc.NewFile(p, gen.fileReg)
	if err != nil {
		return nil, fmt.Errorf("invalid FileDescriptorProto %q: %v", p.GetName(), err)
	}

	if err := gen.fileReg.RegisterFile(desc); err != nil {
		return nil, fmt.Errorf("cannot register descriptor %q: %v", p.GetName(), err)
	}

	f := &File{
		Desc:     desc,
		Proto:    p,
		Location: Location{SourceFile: desc.Path()},
	}

	for i, eds := 0, desc.Enums(); i < eds.Len(); i++ {
		f.Enums = append(f.Enums, newEnum(gen, f, nil, eds.Get(i)))
	}

	for i, mds := 0, desc.Messages(); i < mds.Len(); i++ {
		f.Messages = append(f.Messages, newMessage(gen, f, nil, mds.Get(i)))
	}

	for i, xds := 0, desc.Extensions(); i < xds.Len(); i++ {
		f.Extensions = append(f.Extensions, newField(gen, f, nil, xds.Get(i)))
	}

	for i, sds := 0, desc.Services(); i < sds.Len(); i++ {
		f.Services = append(f.Services, newService(gen, f, sds.Get(i)))
	}

	for _, message := range f.Messages {
		if err := message.resolveDependencies(gen); err != nil {
			return nil, err
		}
	}

	for _, extension := range f.Extensions {
		if err := extension.resolveDependencies(gen); err != nil {
			return nil, err
		}
	}

	for _, service := range f.Services {
		for _, method := range service.Methods {
			if err := method.resolveDependencies(gen); err != nil {
				return nil, err
			}
		}
	}

	return f, nil
}

// An Enum describes an enum.
type Enum struct {
	Desc protoreflect.EnumDescriptor

	Values   []*EnumValue // enum value declarations
	Location Location     // location of this enum
	Comments CommentSet   // comments associated with this enum
}

func newEnum(gen *Generator, f *File, parent *Message, desc protoreflect.EnumDescriptor) *Enum {
	var loc Location
	if parent != nil {
		loc = parent.Location.appendPath(Descriptorproto_EnumType_FieldNumber, desc.Index())
	} else {
		loc = f.Location.appendPath(FileDescriptorProto_EnumType_FieldNumber, desc.Index())
	}

	enum := &Enum{
		Desc:     desc,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	gen.enumsByName[desc.FullName()] = enum

	for i, vds := 0, enum.Desc.Values(); i < vds.Len(); i++ {
		enum.Values = append(enum.Values, newEnumValue(gen, f, parent, enum, vds.Get(i)))
	}

	return enum
}

// An EnumValue describes an enum value.
type EnumValue struct {
	Desc protoreflect.EnumValueDescriptor

	Parent *Enum // enum in which this value is declared

	Location Location   // location of this enum value
	Comments CommentSet // comments associated with this enum value
}

func newEnumValue(gen *Generator, f *File, message *Message, enum *Enum, desc protoreflect.EnumValueDescriptor) *EnumValue {
	loc := enum.Location.appendPath(Enumdescriptorproto_Value_FieldNumber, desc.Index())
	return &EnumValue{
		Desc:     desc,
		Parent:   enum,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
}

// A Message describes a message.
type Message struct {
	Desc protoreflect.MessageDescriptor

	Fields []*Field // message field declarations
	Oneofs []*Oneof // message oneof declarations

	Enums      []*Enum      // nested enum declarations
	Messages   []*Message   // nested message declarations
	Extensions []*Extension // nested extension declarations

	Location Location   // location of this message
	Comments CommentSet // comments associated with this message
}

func newMessage(gen *Generator, f *File, parent *Message, desc protoreflect.MessageDescriptor) *Message {
	var loc Location
	if parent != nil {
		loc = parent.Location.appendPath(Descriptorproto_NestedType_FieldNumber, desc.Index())
	} else {
		loc = f.Location.appendPath(FileDescriptorProto_MessageType_FieldNumber, desc.Index())
	}

	message := &Message{
		Desc:     desc,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	gen.messagesByName[desc.FullName()] = message

	for i, eds := 0, desc.Enums(); i < eds.Len(); i++ {
		message.Enums = append(message.Enums, newEnum(gen, f, message, eds.Get(i)))
	}

	for i, mds := 0, desc.Messages(); i < mds.Len(); i++ {
		message.Messages = append(message.Messages, newMessage(gen, f, message, mds.Get(i)))
	}

	for i, fds := 0, desc.Fields(); i < fds.Len(); i++ {
		message.Fields = append(message.Fields, newField(gen, f, message, fds.Get(i)))
	}

	for i, ods := 0, desc.Oneofs(); i < ods.Len(); i++ {
		message.Oneofs = append(message.Oneofs, newOneof(gen, f, message, ods.Get(i)))
	}

	for i, xds := 0, desc.Extensions(); i < xds.Len(); i++ {
		message.Extensions = append(message.Extensions, newField(gen, f, message, xds.Get(i)))
	}

	// Resolve local references between fields and oneofs.
	for _, field := range message.Fields {
		if od := field.Desc.ContainingOneof(); od != nil {
			oneof := message.Oneofs[od.Index()]
			field.Oneof = oneof
			oneof.Fields = append(oneof.Fields, field)
		}
	}

	return message
}

func (message *Message) resolveDependencies(gen *Generator) error {
	for _, field := range message.Fields {
		if err := field.resolveDependencies(gen); err != nil {
			return err
		}
	}

	for _, message := range message.Messages {
		if err := message.resolveDependencies(gen); err != nil {
			return err
		}
	}

	for _, extension := range message.Extensions {
		if err := extension.resolveDependencies(gen); err != nil {
			return err
		}
	}

	return nil
}

// A Field describes a message field.
type Field struct {
	Desc protoreflect.FieldDescriptor

	Parent *Message // message in which this field is declared; nil if top-level extension

	Oneof    *Oneof   // containing oneof; nil if not part of a oneof
	Extendee *Message // extended message for extension fields; nil otherwise
	Enum     *Enum    // type for enum fields; nil otherwise
	Message  *Message // type for message or group fields; nil otherwise

	Location Location   // location of this field
	Comments CommentSet // comments associated with this field
}

func newField(gen *Generator, f *File, message *Message, desc protoreflect.FieldDescriptor) *Field {
	var loc Location
	switch {
	case desc.IsExtension() && message == nil:
		loc = f.Location.appendPath(FileDescriptorProto_Extension_FieldNumber, desc.Index())
	case desc.IsExtension() && message != nil:
		loc = message.Location.appendPath(Descriptorproto_Extension_FieldNumber, desc.Index())
	default:
		loc = message.Location.appendPath(Descriptorproto_Field_FieldNumber, desc.Index())
	}

	field := &Field{
		Desc:     desc,
		Parent:   message,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}

	return field
}

func (field *Field) resolveDependencies(gen *Generator) error {
	desc := field.Desc

	switch desc.Kind() {
	case protoreflect.EnumKind:
		name := field.Desc.Enum().FullName()
		enum, ok := gen.enumsByName[name]
		if !ok {
			return fmt.Errorf("field %v: no descriptor for enum %v", desc.FullName(), name)
		}
		field.Enum = enum
	case protoreflect.MessageKind, protoreflect.GroupKind:
		name := desc.Message().FullName()
		message, ok := gen.messagesByName[name]
		if !ok {
			return fmt.Errorf("field %v: no descriptor for type %v", desc.FullName(), name)
		}
		field.Message = message
	}

	if desc.IsExtension() {
		name := desc.ContainingMessage().FullName()
		message, ok := gen.messagesByName[name]
		if !ok {
			return fmt.Errorf("field %v: no descriptor for type %v", desc.FullName(), name)
		}
		field.Extendee = message
	}
	return nil
}

// A Oneof describes a message oneof.
type Oneof struct {
	Desc protoreflect.OneofDescriptor

	Parent *Message // message in which this oneof is declared

	Fields []*Field // fields that are part of this oneof

	Location Location   // location of this oneof
	Comments CommentSet // comments associated with this oneof
}

func newOneof(gen *Generator, f *File, message *Message, desc protoreflect.OneofDescriptor) *Oneof {
	loc := message.Location.appendPath(Descriptorproto_OneofDecl_FieldNumber, desc.Index())
	return &Oneof{
		Desc:     desc,
		Parent:   message,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
}

// Extension is an alias of [Field] for documentation.
type Extension = Field

// A Service describes a service.
type Service struct {
	Desc protoreflect.ServiceDescriptor

	Methods  []*Method  // service method declarations
	Location Location   // location of this service
	Comments CommentSet // comments associated with this service
}

func newService(gen *Generator, f *File, desc protoreflect.ServiceDescriptor) *Service {
	loc := f.Location.appendPath(FileDescriptorProto_Service_FieldNumber, desc.Index())
	service := &Service{
		Desc:     desc,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}

	for i, mds := 0, desc.Methods(); i < mds.Len(); i++ {
		service.Methods = append(service.Methods, newMethod(gen, f, service, mds.Get(i)))
	}

	return service
}

// A Method describes a method in a service.
type Method struct {
	Desc protoreflect.MethodDescriptor

	Parent *Service // service in which this method is declared

	Input  *Message
	Output *Message

	Location Location   // location of this method
	Comments CommentSet // comments associated with this method
}

func newMethod(gen *Generator, f *File, service *Service, desc protoreflect.MethodDescriptor) *Method {
	loc := service.Location.appendPath(Servicedescriptorproto_Method_FieldNumber, desc.Index())
	method := &Method{
		Desc:     desc,
		Parent:   service,
		Location: loc,
		Comments: makeCommentSet(f.Desc.SourceLocations().ByDescriptor(desc)),
	}
	return method
}

func (method *Method) resolveDependencies(gen *Generator) error {
	desc := method.Desc

	inName := desc.Input().FullName()
	in, ok := gen.messagesByName[inName]
	if !ok {
		return fmt.Errorf("method %v: no descriptor for type %v", desc.FullName(), inName)
	}
	method.Input = in

	outName := desc.Output().FullName()
	out, ok := gen.messagesByName[outName]
	if !ok {
		return fmt.Errorf("method %v: no descriptor for type %v", desc.FullName(), outName)
	}
	method.Output = out

	return nil
}

// A Location is a location in a .proto source file.
//
// See the google.protobuf.SourceCodeInfo documentation in descriptor.proto
// for details.
type Location struct {
	SourceFile string
	Path       protoreflect.SourcePath
}

// appendPath add elements to a Location's path, returning a new Location.
func (loc Location) appendPath(num protoreflect.FieldNumber, idx int) Location {
	loc.Path = append(protoreflect.SourcePath(nil), loc.Path...) // make copy
	loc.Path = append(loc.Path, int32(num), int32(idx))
	return loc
}

// CommentSet is a set of leading and trailing comments associated
// with a .proto descriptor declaration.
type CommentSet struct {
	LeadingDetached []Comments
	Leading         Comments
	Trailing        Comments
}

func makeCommentSet(loc protoreflect.SourceLocation) CommentSet {
	var leadingDetached []Comments
	for _, s := range loc.LeadingDetachedComments {
		leadingDetached = append(leadingDetached, Comments(s))
	}

	return CommentSet{
		LeadingDetached: leadingDetached,
		Leading:         Comments(loc.LeadingComments),
		Trailing:        Comments(loc.TrailingComments),
	}
}

// Comments is a comments string as provided by protoc.
type Comments string

// String formats the comments by inserting // to the start of each line,
// ensuring that there is a trailing newline.
// An empty comment is formatted as an empty string.
func (c Comments) String() string {
	if c == "" {
		return ""
	}
	var b []byte
	for _, line := range strings.Split(strings.TrimSuffix(string(c), "\n"), "\n") {
		b = append(b, "//"...)
		b = append(b, line...)
		b = append(b, "\n"...)
	}
	return string(b)
}
