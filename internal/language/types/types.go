package types

import (
	"fmt"
	"sort"
	"strings"
)

// TypeKind identifies the semantic category of a type.
type TypeKind int

const (
	KindPrimitive TypeKind = iota
	KindContent
	KindList
	KindMap
	KindRecord
	KindOptional
	KindUnion
	KindQuantity
	KindEntity
	KindAnyContent
	KindUnknown
	KindVoid
	KindJsonValue
	KindJsonNull
)

// Type represents a canonical semantic type in Cherri language v2.0.
type Type interface {
	Kind() TypeKind
	Name() string
	String() string
	AssignableTo(target Type) bool
	Equals(other Type) bool
}

// Base primitive types
type PrimitiveType struct {
	name string
}

func (p PrimitiveType) Kind() TypeKind             { return KindPrimitive }
func (p PrimitiveType) Name() string               { return p.name }
func (p PrimitiveType) String() string             { return p.name }
func (p PrimitiveType) Equals(other Type) bool {
	if o, ok := other.(PrimitiveType); ok {
		return p.name == o.name
	}
	return false
}
func (p PrimitiveType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	if opt, ok := target.(OptionalType); ok {
		return p.AssignableTo(opt.Inner)
	}
	if u, ok := target.(UnionType); ok {
		for _, m := range u.Members {
			if p.AssignableTo(m) {
				return true
			}
		}
		return false
	}
	return p.Equals(target)
}

var (
	Text   = PrimitiveType{name: "Text"}
	Bool   = PrimitiveType{name: "Bool"}
	Number = PrimitiveType{name: "Number"}
)

// ContentType represents an Apple Shortcuts content type (Image, File, URL, Date, Contact, CalendarEvent, etc.)
type ContentType struct {
	name string
}

func (c ContentType) Kind() TypeKind             { return KindContent }
func (c ContentType) Name() string               { return c.name }
func (c ContentType) String() string             { return c.name }
func (c ContentType) Equals(other Type) bool {
	if o, ok := other.(ContentType); ok {
		return c.name == o.name
	}
	return false
}
func (c ContentType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	if opt, ok := target.(OptionalType); ok {
		return c.AssignableTo(opt.Inner)
	}
	if u, ok := target.(UnionType); ok {
		for _, m := range u.Members {
			if c.AssignableTo(m) {
				return true
			}
		}
		return false
	}
	return c.Equals(target)
}

var (
	Image         = ContentType{name: "Image"}
	File          = ContentType{name: "File"}
	URL           = ContentType{name: "URL"}
	Date          = ContentType{name: "Date"}
	Contact       = ContentType{name: "Contact"}
	CalendarEvent = ContentType{name: "CalendarEvent"}
)

// AnyContent represents arbitrary Apple content that any content item or primitive can satisfy.
type anyContentType struct{}

func (anyContentType) Kind() TypeKind                 { return KindAnyContent }
func (anyContentType) Name() string                   { return "AnyContent" }
func (anyContentType) String() string                 { return "AnyContent" }
func (anyContentType) Equals(other Type) bool         { return other.Kind() == KindAnyContent }
func (anyContentType) AssignableTo(target Type) bool {
	return target.Kind() == KindAnyContent || target.Kind() == KindUnknown
}

var AnyContent = anyContentType{}

// UnknownType represents missing or unverified type knowledge.
type unknownType struct{}

func (unknownType) Kind() TypeKind                 { return KindUnknown }
func (unknownType) Name() string                   { return "Unknown" }
func (unknownType) String() string                 { return "Unknown" }
func (unknownType) Equals(other Type) bool         { return other.Kind() == KindUnknown }
func (unknownType) AssignableTo(target Type) bool  { return true } // Unknown is conservatively accepted

var Unknown = unknownType{}

// VoidType represents no return value.
type voidType struct{}

func (voidType) Kind() TypeKind                 { return KindVoid }
func (voidType) Name() string                   { return "Void" }
func (voidType) String() string                 { return "Void" }
func (voidType) Equals(other Type) bool         { return other.Kind() == KindVoid }
func (voidType) AssignableTo(target Type) bool  { return target.Kind() == KindVoid }

var Void = voidType{}

// JsonValueType and JsonNullType for loss-preserving external JSON
type jsonValueType struct{}

func (jsonValueType) Kind() TypeKind                 { return KindJsonValue }
func (jsonValueType) Name() string                   { return "JsonValue" }
func (jsonValueType) String() string                 { return "JsonValue" }
func (jsonValueType) Equals(other Type) bool         { return other.Kind() == KindJsonValue }
func (jsonValueType) AssignableTo(target Type) bool {
	return target.Kind() == KindJsonValue || target.Kind() == KindUnknown || target.Kind() == KindAnyContent
}

var JsonValue = jsonValueType{}

type jsonNullType struct{}

func (jsonNullType) Kind() TypeKind                 { return KindJsonNull }
func (jsonNullType) Name() string                   { return "JsonNull" }
func (jsonNullType) String() string                 { return "JsonNull" }
func (jsonNullType) Equals(other Type) bool         { return other.Kind() == KindJsonNull }
func (jsonNullType) AssignableTo(target Type) bool {
	return target.Kind() == KindJsonNull || target.Kind() == KindJsonValue || target.Kind() == KindUnknown
}

var JsonNull = jsonNullType{}

// OptionalType represents T? (optional value).
type OptionalType struct {
	Inner Type
}

func NewOptional(inner Type) Type {
	if inner == nil || inner.Kind() == KindUnknown {
		return Unknown
	}
	if opt, ok := inner.(OptionalType); ok {
		return opt // flatten T?? to T?
	}
	return OptionalType{Inner: inner}
}

func (o OptionalType) Kind() TypeKind { return KindOptional }
func (o OptionalType) Name() string   { return o.Inner.Name() + "?" }
func (o OptionalType) String() string { return o.Inner.String() + "?" }
func (o OptionalType) Equals(other Type) bool {
	if opt, ok := other.(OptionalType); ok {
		return o.Inner.Equals(opt.Inner)
	}
	return false
}
func (o OptionalType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	if opt, ok := target.(OptionalType); ok {
		return o.Inner.AssignableTo(opt.Inner)
	}
	if u, ok := target.(UnionType); ok {
		for _, m := range u.Members {
			if o.AssignableTo(m) {
				return true
			}
		}
	}
	return false
}

// ListType represents List<T>.
type ListType struct {
	Element Type
}

func NewList(element Type) ListType {
	if element == nil {
		element = Unknown
	}
	return ListType{Element: element}
}

func (l ListType) Kind() TypeKind { return KindList }
func (l ListType) Name() string   { return fmt.Sprintf("List<%s>", l.Element.Name()) }
func (l ListType) String() string { return fmt.Sprintf("List<%s>", l.Element.String()) }
func (l ListType) Equals(other Type) bool {
	if o, ok := other.(ListType); ok {
		return l.Element.Equals(o.Element)
	}
	return false
}
func (l ListType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	if opt, ok := target.(OptionalType); ok {
		return l.AssignableTo(opt.Inner)
	}
	if o, ok := target.(ListType); ok {
		return l.Element.AssignableTo(o.Element)
	}
	if u, ok := target.(UnionType); ok {
		for _, m := range u.Members {
			if l.AssignableTo(m) {
				return true
			}
		}
	}
	return false
}

// MapType represents Map<Text, V>.
type MapType struct {
	Key   Type
	Value Type
}

func NewMap(key, value Type) MapType {
	if key == nil {
		key = Text
	}
	if value == nil {
		value = Unknown
	}
	return MapType{Key: key, Value: value}
}

func (m MapType) Kind() TypeKind { return KindMap }
func (m MapType) Name() string   { return fmt.Sprintf("Map<%s, %s>", m.Key.Name(), m.Value.Name()) }
func (m MapType) String() string { return fmt.Sprintf("Map<%s, %s>", m.Key.String(), m.Value.String()) }
func (m MapType) Equals(other Type) bool {
	if o, ok := other.(MapType); ok {
		return m.Key.Equals(o.Key) && m.Value.Equals(o.Value)
	}
	return false
}
func (m MapType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	if opt, ok := target.(OptionalType); ok {
		return m.AssignableTo(opt.Inner)
	}
	if o, ok := target.(MapType); ok {
		return m.Key.AssignableTo(o.Key) && m.Value.AssignableTo(o.Value)
	}
	if u, ok := target.(UnionType); ok {
		for _, member := range u.Members {
			if m.AssignableTo(member) {
				return true
			}
		}
	}
	return false
}

// RecordField defines a field in a record type.
type RecordField struct {
	Name     string
	Type     Type
	Optional bool // field present?
}

// RecordType represents structural record types: { field: Type, optionalField?: Type }.
type RecordType struct {
	Fields map[string]RecordField
}

func NewRecord(fields []RecordField) RecordType {
	m := make(map[string]RecordField, len(fields))
	for _, f := range fields {
		m[f.Name] = f
	}
	return RecordType{Fields: m}
}

func (r RecordType) Kind() TypeKind { return KindRecord }
func (r RecordType) Name() string   { return r.String() }
func (r RecordType) String() string {
	keys := make([]string, 0, len(r.Fields))
	for k := range r.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		f := r.Fields[k]
		opt := ""
		if f.Optional {
			opt = "?"
		}
		parts = append(parts, fmt.Sprintf("%s%s: %s", f.Name, opt, f.Type.String()))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
func (r RecordType) Equals(other Type) bool {
	o, ok := other.(RecordType)
	if !ok || len(r.Fields) != len(o.Fields) {
		return false
	}
	for k, rf := range r.Fields {
		of, exists := o.Fields[k]
		if !exists || rf.Optional != of.Optional || !rf.Type.Equals(of.Type) {
			return false
		}
	}
	return true
}
func (r RecordType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	if opt, ok := target.(OptionalType); ok {
		return r.AssignableTo(opt.Inner)
	}
	if o, ok := target.(RecordType); ok {
		// Target's required fields must be satisfied
		for k, tf := range o.Fields {
			sf, exists := r.Fields[k]
			if !exists {
				if !tf.Optional {
					return false
				}
				continue
			}
			if !sf.Type.AssignableTo(tf.Type) {
				return false
			}
		}
		return true
	}
	return false
}

// UnionType represents T | U.
type UnionType struct {
	Members []Type
}

func NewUnion(members ...Type) Type {
	flattened := make([]Type, 0, len(members))
	for _, m := range members {
		if m == nil {
			continue
		}
		if u, ok := m.(UnionType); ok {
			flattened = append(flattened, u.Members...)
		} else {
			flattened = append(flattened, m)
		}
	}
	// Deduplicate
	unique := make([]Type, 0, len(flattened))
	for _, m := range flattened {
		dup := false
		for _, u := range unique {
			if u.Equals(m) {
				dup = true
				break
			}
		}
		if !dup {
			unique = append(unique, m)
		}
	}
	if len(unique) == 0 {
		return Unknown
	}
	if len(unique) == 1 {
		return unique[0]
	}
	return UnionType{Members: unique}
}

func (u UnionType) Kind() TypeKind { return KindUnion }
func (u UnionType) Name() string   { return u.String() }
func (u UnionType) String() string {
	names := make([]string, len(u.Members))
	for i, m := range u.Members {
		names[i] = m.String()
	}
	return strings.Join(names, " | ")
}
func (u UnionType) Equals(other Type) bool {
	o, ok := other.(UnionType)
	if !ok || len(u.Members) != len(o.Members) {
		return false
	}
	for _, m := range u.Members {
		found := false
		for _, om := range o.Members {
			if m.Equals(om) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func (u UnionType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	// Every member of u must be assignable to target
	for _, m := range u.Members {
		if !m.AssignableTo(target) {
			return false
		}
	}
	return true
}

// EntityType represents nominal app entities such as Notes.Note
type EntityType struct {
	Namespace string
	NameStr   string
}

func NewEntity(namespace, name string) EntityType {
	return EntityType{Namespace: namespace, NameStr: name}
}

func (e EntityType) Kind() TypeKind { return KindEntity }
func (e EntityType) Name() string {
	if e.Namespace != "" {
		return e.Namespace + "." + e.NameStr
	}
	return e.NameStr
}
func (e EntityType) String() string { return e.Name() }
func (e EntityType) Equals(other Type) bool {
	if o, ok := other.(EntityType); ok {
		return e.Namespace == o.Namespace && e.NameStr == o.NameStr
	}
	return false
}
func (e EntityType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	return e.Equals(target)
}

// QuantityType represents values with a unit family (length, mass, duration, etc.)
type QuantityType struct {
	UnitFamily string
}

func (q QuantityType) Kind() TypeKind { return KindQuantity }
func (q QuantityType) Name() string   { return fmt.Sprintf("Quantity<%s>", q.UnitFamily) }
func (q QuantityType) String() string { return q.Name() }
func (q QuantityType) Equals(other Type) bool {
	if o, ok := other.(QuantityType); ok {
		return q.UnitFamily == o.UnitFamily
	}
	return false
}
func (q QuantityType) AssignableTo(target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return true
	}
	return q.Equals(target)
}

// ParseNamedType resolves a type name to a standard canonical Type.
func ParseNamedType(name string) (Type, bool) {
	switch name {
	case "Text", "text", "String", "string":
		return Text, true
	case "Bool", "bool", "Boolean", "boolean":
		return Bool, true
	case "Number", "number", "Int", "int", "Float", "float", "Integer", "integer":
		return Number, true
	case "Image", "image":
		return Image, true
	case "File", "file":
		return File, true
	case "URL", "url":
		return URL, true
	case "Date", "date":
		return Date, true
	case "Contact", "contact":
		return Contact, true
	case "CalendarEvent", "calendarEvent", "event":
		return CalendarEvent, true
	case "AnyContent", "any", "Variable", "variable":
		return AnyContent, true
	case "Unknown", "unknown":
		return Unknown, true
	case "Void", "void":
		return Void, true
	case "JsonValue":
		return JsonValue, true
	case "JsonNull":
		return JsonNull, true
	default:
		return nil, false
	}
}
