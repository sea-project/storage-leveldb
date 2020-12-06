package leveldb

import (
	"github.com/sea-project/storage-leveldb/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"sync"
)

type LevelDB struct {
	fn string      // 数据库路径
	db *leveldb.DB // 数据库句柄
}

var db *LevelDB
var err error
var once sync.Once

func Init(file string) *LevelDB {
	once.Do(func() {
		db, err = NewDB(file)
		if err != nil {
			panic(err)
		}
	})
	return db
}

func NewDB(file string) (*LevelDB, error) {
	// 打开数据库并定义相关参数
	db, err := leveldb.OpenFile(file, &opt.Options{
		Compression:         opt.SnappyCompression,
		WriteBuffer:         64 * opt.MiB,
		CompactionTableSize: 2 * opt.MiB,               // 定义数据文件最大存储
		Filter:              filter.NewBloomFilter(10), // bloom过滤器
	})
	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}
	if err != nil {
		return nil, err
	}

	// 结构体赋值并返回
	return &LevelDB{fn: file, db: db}, nil
}

// 返回数据库路径
func (db *LevelDB) Path() string {
	return db.fn
}

// 数据库写操作
func (db *LevelDB) Put(key []byte, value []byte) error {
	return db.db.Put(key, value, nil)
}

// 数据库读操作
func (db *LevelDB) Get(key []byte) ([]byte, error) {
	data, err := db.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// 数据库删除操作
func (db *LevelDB) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

// 返回某KEY是否存在
func (db *LevelDB) Has(key []byte) (bool, error) {
	return db.db.Has(key, nil)
}

// 数据库迭代器
func (db *LevelDB) NewIterator() iterator.Iterator {
	return db.db.NewIterator(nil, nil)
}

// NewIteratorWithStart creates a binary-alphabetical iterator over a subset of
// database content starting at a particular initial key (or after, if it does
// not exist).
func (db *LevelDB) NewIteratorWithStart(start []byte) iterator.Iterator {
	return db.db.NewIterator(&util.Range{Start: start}, nil)
}

// 返回数据库句柄
func (db *LevelDB) GetDB() *leveldb.DB {
	return db.db
}

// 关闭数据库
func (db *LevelDB) Close() error {
	if err := db.db.Close(); err != nil {
		return err
	}
	return nil
}

// 定义批量存储结构体
type LdbBatch struct {
	db    *leveldb.DB
	batch *leveldb.Batch
	size  int
}

// 初始化批量存储
func (db *LevelDB) NewBatch() types.Batch {
	return &LdbBatch{
		db:    db.db,
		batch: new(leveldb.Batch),
	}
}

// 写入暂存区
func (b *LdbBatch) Put(key, value []byte) error {
	b.batch.Put(key, value)
	b.size += len(value)
	return nil
}

// 批量写入数据库
func (b *LdbBatch) Write() error {
	return b.db.Write(b.batch, nil)
}

// batch Delete
func (b *LdbBatch) Delete(key []byte) error {
	b.batch.Delete(key)
	b.size++
	return nil
}

// batch ValueSize
func (b *LdbBatch) ValueSize() int {
	return b.size
}

// batch Reset
func (b *LdbBatch) Reset() {
	b.batch.Reset()
	b.size = 0
}

// batch Replay
func (b *LdbBatch) Replay(w types.Putter) error {
	return b.batch.Replay(&replayer{writer: w})
}

// replayer
type replayer struct {
	writer  types.Putter
	failure error
}

// replayer Put
func (r *replayer) Put(key, value []byte) {
	// If the replay already failed, stop executing ops
	if r.failure != nil {
		return
	}
	r.failure = r.writer.Put(key, value)
}

// replayer Delete
func (r *replayer) Delete(key []byte) {
	// If the replay already failed, stop executing ops
	if r.failure != nil {
		return
	}
	r.failure = r.writer.Delete(key)
}
