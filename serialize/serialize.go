package serialize

// Serializer is an interface for serializing objects as JSON to a path.
type Serializer interface {
	Serialize(obj interface{}, path string) error
}
