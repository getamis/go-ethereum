package pbft

type Dber interface {
	// Save an object into db
	Save(key string, val interface{}) error
	// Restore an object to val from db
	Restore(key string, val interface{}) error
}
