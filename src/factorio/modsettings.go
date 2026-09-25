package factorio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

// ModSettingsFileName is the name of the Factorio mod settings file. It is
// stored inside the mods directory next to mod-list.json.
const ModSettingsFileName = "mod-settings.dat"

// minModSettingsVersion is the oldest settings file version this codec
// supports. Older files use a different (non optimized) string encoding.
var minModSettingsVersion = Version{0, 18, 0, 0}

// PropertyTreeType is the type tag of a PropertyTree node.
type PropertyTreeType uint8

const (
	PropertyTreeTypeNull PropertyTreeType = iota
	PropertyTreeTypeBool
	PropertyTreeTypeNumber
	PropertyTreeTypeString
	PropertyTreeTypeList
	PropertyTreeTypeDictionary
	PropertyTreeTypeSignedInt
	PropertyTreeTypeUnsignedInt
)

// PropertyTree is one node of Factorio's PropertyTree serialization format,
// which is used by mod-settings.dat, level-init.dat and script.dat.
//
// Dictionary and list entries store their key on the child node, matching the
// binary layout (key first, then the node itself).
type PropertyTree struct {
	Key         *string // nil means the key is absent (immutable string None)
	ValueType   PropertyTreeType
	AnyType     uint8 // raw any-type byte, preserved as-is
	Bool        bool
	Number      float64
	String      *string // nil means the value is absent (immutable string None)
	Children    []PropertyTree
	SignedInt   int64
	UnsignedInt uint64
}

// ModSettings is the decoded content of a mod-settings.dat file.
type ModSettings struct {
	Version    Version
	HasQuality bool
	Data       PropertyTree
}

// DictionaryChild returns the first child whose key matches key, or nil when
// there is no such child. Keys are attached to the child nodes.
func (t *PropertyTree) DictionaryChild(key string) *PropertyTree {
	if t == nil {
		return nil
	}
	for i := range t.Children {
		child := &t.Children[i]
		if child.Key != nil && *child.Key == key {
			return child
		}
	}
	return nil
}

// SettingValue returns the "value" node of the named setting inside a
// top-level section such as "startup" or "runtime-global".
func (m *ModSettings) SettingValue(section, name string) (*PropertyTree, error) {
	sectionNode := m.Data.DictionaryChild(section)
	if sectionNode == nil {
		return nil, fmt.Errorf("section %q not found", section)
	}

	settingNode := sectionNode.DictionaryChild(name)
	if settingNode == nil {
		return nil, fmt.Errorf("setting %q not found in section %q", name, section)
	}

	valueNode := settingNode.DictionaryChild("value")
	if valueNode == nil {
		return nil, fmt.Errorf("setting %q in section %q has no value", name, section)
	}

	return valueNode, nil
}

// ReadModSettings decodes a mod-settings.dat stream.
func ReadModSettings(r io.Reader) (*ModSettings, error) {
	var rawVersion [4]uint16
	for i := range rawVersion {
		var b [2]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, fmt.Errorf("read mod settings version: %v", err)
		}
		rawVersion[i] = binary.LittleEndian.Uint16(b[:])
	}

	var hasQuality [1]byte
	if _, err := io.ReadFull(r, hasQuality[:]); err != nil {
		return nil, fmt.Errorf("read mod settings header: %v", err)
	}

	settings := &ModSettings{
		Version:    Version{uint(rawVersion[0]), uint(rawVersion[1]), uint(rawVersion[2]), uint(rawVersion[3])},
		HasQuality: hasQuality[0] != 0,
	}

	if settings.Version.Less(minModSettingsVersion) {
		return nil, fmt.Errorf("unsupported mod settings version %s, 0.18.0.0 or newer is required", settings.Version)
	}

	data, err := readPropertyTree(r)
	if err != nil {
		return nil, fmt.Errorf("read mod settings data: %v", err)
	}
	settings.Data = *data

	return settings, nil
}

// WriteTo encodes the mod settings into the mod-settings.dat format.
func (m *ModSettings) WriteTo(w io.Writer) error {
	var b [4]byte
	for i := range m.Version {
		binary.LittleEndian.PutUint16(b[:2], uint16(m.Version[i]))
		if _, err := w.Write(b[:2]); err != nil {
			return err
		}
	}

	if m.HasQuality {
		b[0] = 1
	} else {
		b[0] = 0
	}
	if _, err := w.Write(b[:1]); err != nil {
		return err
	}

	return writePropertyTree(w, &m.Data)
}

// LoadModSettingsFile reads and decodes a mod-settings.dat file from disk.
func LoadModSettingsFile(path string) (*ModSettings, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	settings, err := ReadModSettings(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}

	return settings, nil
}

// SaveModSettingsFile encodes the settings and atomically replaces the file at
// path, keeping its previous permissions when it already exists.
func SaveModSettingsFile(path string, settings *ModSettings) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	if info, err := os.Stat(path); err == nil {
		if err := tmp.Chmod(info.Mode().Perm()); err != nil {
			tmp.Close()
			return fmt.Errorf("set permissions on %s: %v", tmpName, err)
		}
	}

	if err := settings.WriteTo(tmp); err != nil {
		tmp.Close()
		return fmt.Errorf("encode %s: %v", path, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %v", tmpName, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %v", path, err)
	}
	tmpName = ""

	return nil
}

func readPropertyTree(r io.Reader) (*PropertyTree, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, fmt.Errorf("read property tree node header: %v", err)
	}

	node := &PropertyTree{
		ValueType: PropertyTreeType(header[0]),
		AnyType:   header[1],
	}

	switch node.ValueType {
	case PropertyTreeTypeNull:
	case PropertyTreeTypeBool:
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, fmt.Errorf("read bool: %v", err)
		}
		node.Bool = b[0] != 0
	case PropertyTreeTypeNumber:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, fmt.Errorf("read number: %v", err)
		}
		node.Number = math.Float64frombits(binary.LittleEndian.Uint64(b[:]))
	case PropertyTreeTypeString:
		value, err := readImmutableString(r)
		if err != nil {
			return nil, err
		}
		node.String = value
	case PropertyTreeTypeList, PropertyTreeTypeDictionary:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, fmt.Errorf("read container length: %v", err)
		}
		count := binary.LittleEndian.Uint32(b[:])
		if count > 0 {
			node.Children = make([]PropertyTree, 0, count)
		}
		for i := uint32(0); i < count; i++ {
			key, err := readImmutableString(r)
			if err != nil {
				return nil, fmt.Errorf("read container entry %d: %v", i, err)
			}
			child, err := readPropertyTree(r)
			if err != nil {
				return nil, fmt.Errorf("read container entry %d: %v", i, err)
			}
			child.Key = key
			node.Children = append(node.Children, *child)
		}
	case PropertyTreeTypeSignedInt:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, fmt.Errorf("read signed int: %v", err)
		}
		node.SignedInt = int64(binary.LittleEndian.Uint64(b[:]))
	case PropertyTreeTypeUnsignedInt:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, fmt.Errorf("read unsigned int: %v", err)
		}
		node.UnsignedInt = binary.LittleEndian.Uint64(b[:])
	default:
		return nil, fmt.Errorf("unknown property tree value type %d", header[0])
	}

	return node, nil
}

func writePropertyTree(w io.Writer, node *PropertyTree) error {
	if _, err := w.Write([]byte{byte(node.ValueType), node.AnyType}); err != nil {
		return err
	}

	switch node.ValueType {
	case PropertyTreeTypeNull:
	case PropertyTreeTypeBool:
		var b [1]byte
		if node.Bool {
			b[0] = 1
		}
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
	case PropertyTreeTypeNumber:
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(node.Number))
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
	case PropertyTreeTypeString:
		if err := writeImmutableString(w, node.String); err != nil {
			return err
		}
	case PropertyTreeTypeList, PropertyTreeTypeDictionary:
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(len(node.Children)))
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
		for i := range node.Children {
			child := &node.Children[i]
			if err := writeImmutableString(w, child.Key); err != nil {
				return err
			}
			if err := writePropertyTree(w, child); err != nil {
				return err
			}
		}
	case PropertyTreeTypeSignedInt:
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], uint64(node.SignedInt))
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
	case PropertyTreeTypeUnsignedInt:
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], node.UnsignedInt)
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown property tree value type %d", node.ValueType)
	}

	return nil
}

// readImmutableString reads the format's length prefixed string type. A
// non-zero leading byte marks the value as absent.
func readImmutableString(r io.Reader) (*string, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, fmt.Errorf("read string presence: %v", err)
	}
	if b[0] != 0 {
		return nil, nil
	}

	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, fmt.Errorf("read string length: %v", err)
	}
	length := uint32(b[0])
	if b[0] == 0xFF {
		var b4 [4]byte
		if _, err := io.ReadFull(r, b4[:]); err != nil {
			return nil, fmt.Errorf("read string length: %v", err)
		}
		length = binary.LittleEndian.Uint32(b4[:])
	}

	value := make([]byte, length)
	if _, err := io.ReadFull(r, value); err != nil {
		return nil, fmt.Errorf("read string of length %d: %v", length, err)
	}

	s := string(value)
	return &s, nil
}

func writeImmutableString(w io.Writer, value *string) error {
	if value == nil {
		_, err := w.Write([]byte{1})
		return err
	}

	if _, err := w.Write([]byte{0}); err != nil {
		return err
	}

	if len(*value) >= 0xFF {
		var b [5]byte
		b[0] = 0xFF
		binary.LittleEndian.PutUint32(b[1:], uint32(len(*value)))
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
	} else {
		if _, err := w.Write([]byte{byte(len(*value))}); err != nil {
			return err
		}
	}

	_, err := io.WriteString(w, *value)
	return err
}
