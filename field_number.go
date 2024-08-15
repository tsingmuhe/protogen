package protogen

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

//https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/descriptor.proto

// Describes a complete .proto file.
const (
	FileDescriptorProto_MessageType_FieldNumber protoreflect.FieldNumber = 4
	FileDescriptorProto_EnumType_FieldNumber    protoreflect.FieldNumber = 5
	FileDescriptorProto_Service_FieldNumber     protoreflect.FieldNumber = 6
	FileDescriptorProto_Extension_FieldNumber   protoreflect.FieldNumber = 7
)

// Describes a message type.
const (
	Descriptorproto_Field_FieldNumber      protoreflect.FieldNumber = 2
	Descriptorproto_Extension_FieldNumber  protoreflect.FieldNumber = 6
	Descriptorproto_NestedType_FieldNumber protoreflect.FieldNumber = 3
	Descriptorproto_EnumType_FieldNumber   protoreflect.FieldNumber = 4
	Descriptorproto_OneofDecl_FieldNumber  protoreflect.FieldNumber = 8
)

// Describes an enum type.
const (
	Enumdescriptorproto_Value_FieldNumber protoreflect.FieldNumber = 2
)

// Describes a service.
const (
	Servicedescriptorproto_Method_FieldNumber protoreflect.FieldNumber = 2
)
