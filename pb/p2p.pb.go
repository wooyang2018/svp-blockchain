// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v4.23.3
// source: p2p.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Request_Type int32

const (
	Request_Invalid       Request_Type = 0
	Request_Block         Request_Type = 1
	Request_QC            Request_Type = 2
	Request_BlockByHeight Request_Type = 3
	Request_TxList        Request_Type = 4
)

// Enum value maps for Request_Type.
var (
	Request_Type_name = map[int32]string{
		0: "Invalid",
		1: "Block",
		2: "QC",
		3: "BlockByHeight",
		4: "TxList",
	}
	Request_Type_value = map[string]int32{
		"Invalid":       0,
		"Block":         1,
		"QC":            2,
		"BlockByHeight": 3,
		"TxList":        4,
	}
)

func (x Request_Type) Enum() *Request_Type {
	p := new(Request_Type)
	*p = x
	return p
}

func (x Request_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Request_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_p2p_proto_enumTypes[0].Descriptor()
}

func (Request_Type) Type() protoreflect.EnumType {
	return &file_p2p_proto_enumTypes[0]
}

func (x Request_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Request_Type.Descriptor instead.
func (Request_Type) EnumDescriptor() ([]byte, []int) {
	return file_p2p_proto_rawDescGZIP(), []int{0, 0}
}

type Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type Request_Type `protobuf:"varint,1,opt,name=type,proto3,enum=p2p.pb.Request_Type" json:"type,omitempty"`
	Data []byte       `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	Seq  uint32       `protobuf:"varint,3,opt,name=seq,proto3" json:"seq,omitempty"`
}

func (x *Request) Reset() {
	*x = Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_p2p_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Request) ProtoMessage() {}

func (x *Request) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Request.ProtoReflect.Descriptor instead.
func (*Request) Descriptor() ([]byte, []int) {
	return file_p2p_proto_rawDescGZIP(), []int{0}
}

func (x *Request) GetType() Request_Type {
	if x != nil {
		return x.Type
	}
	return Request_Invalid
}

func (x *Request) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Request) GetSeq() uint32 {
	if x != nil {
		return x.Seq
	}
	return 0
}

type Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Seq   uint32 `protobuf:"varint,1,opt,name=seq,proto3" json:"seq,omitempty"`
	Data  []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	Error string `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *Response) Reset() {
	*x = Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_p2p_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Response) ProtoMessage() {}

func (x *Response) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Response.ProtoReflect.Descriptor instead.
func (*Response) Descriptor() ([]byte, []int) {
	return file_p2p_proto_rawDescGZIP(), []int{1}
}

func (x *Response) GetSeq() uint32 {
	if x != nil {
		return x.Seq
	}
	return 0
}

func (x *Response) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Response) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type HashList struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	List [][]byte `protobuf:"bytes,1,rep,name=list,proto3" json:"list,omitempty"`
}

func (x *HashList) Reset() {
	*x = HashList{}
	if protoimpl.UnsafeEnabled {
		mi := &file_p2p_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HashList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HashList) ProtoMessage() {}

func (x *HashList) ProtoReflect() protoreflect.Message {
	mi := &file_p2p_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HashList.ProtoReflect.Descriptor instead.
func (*HashList) Descriptor() ([]byte, []int) {
	return file_p2p_proto_rawDescGZIP(), []int{2}
}

func (x *HashList) GetList() [][]byte {
	if x != nil {
		return x.List
	}
	return nil
}

var File_p2p_proto protoreflect.FileDescriptor

var file_p2p_proto_rawDesc = []byte{
	0x0a, 0x09, 0x70, 0x32, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x70, 0x32, 0x70,
	0x2e, 0x70, 0x62, 0x22, 0xa0, 0x01, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x28, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x14, 0x2e,
	0x70, 0x32, 0x70, 0x2e, 0x70, 0x62, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x2e, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x10, 0x0a,
	0x03, 0x73, 0x65, 0x71, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x03, 0x73, 0x65, 0x71, 0x22,
	0x45, 0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x49, 0x6e, 0x76, 0x61, 0x6c,
	0x69, 0x64, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x10, 0x01, 0x12,
	0x06, 0x0a, 0x02, 0x51, 0x43, 0x10, 0x02, 0x12, 0x11, 0x0a, 0x0d, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x42, 0x79, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x10, 0x03, 0x12, 0x0a, 0x0a, 0x06, 0x54, 0x78,
	0x4c, 0x69, 0x73, 0x74, 0x10, 0x04, 0x22, 0x46, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x73, 0x65, 0x71, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x03, 0x73, 0x65, 0x71, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f,
	0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x22, 0x1e,
	0x0a, 0x08, 0x48, 0x61, 0x73, 0x68, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6c, 0x69,
	0x73, 0x74, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x04, 0x6c, 0x69, 0x73, 0x74, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_p2p_proto_rawDescOnce sync.Once
	file_p2p_proto_rawDescData = file_p2p_proto_rawDesc
)

func file_p2p_proto_rawDescGZIP() []byte {
	file_p2p_proto_rawDescOnce.Do(func() {
		file_p2p_proto_rawDescData = protoimpl.X.CompressGZIP(file_p2p_proto_rawDescData)
	})
	return file_p2p_proto_rawDescData
}

var file_p2p_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_p2p_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_p2p_proto_goTypes = []interface{}{
	(Request_Type)(0), // 0: p2p.pb.Request.Type
	(*Request)(nil),   // 1: p2p.pb.Request
	(*Response)(nil),  // 2: p2p.pb.Response
	(*HashList)(nil),  // 3: p2p.pb.HashList
}
var file_p2p_proto_depIdxs = []int32{
	0, // 0: p2p.pb.Request.type:type_name -> p2p.pb.Request.Type
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_p2p_proto_init() }
func file_p2p_proto_init() {
	if File_p2p_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_p2p_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_p2p_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Response); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_p2p_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HashList); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_p2p_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_p2p_proto_goTypes,
		DependencyIndexes: file_p2p_proto_depIdxs,
		EnumInfos:         file_p2p_proto_enumTypes,
		MessageInfos:      file_p2p_proto_msgTypes,
	}.Build()
	File_p2p_proto = out.File
	file_p2p_proto_rawDesc = nil
	file_p2p_proto_goTypes = nil
	file_p2p_proto_depIdxs = nil
}
