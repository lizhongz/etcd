package backend

import (
	"log"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/boltdb/bolt"
)

type Backend interface {
	BatchTx() BatchTx
	ForceCommit()
	Close() error
}

type backend struct {
	db *bolt.DB

	batchInterval time.Duration
	batchLimit    int
	batchTx       *batchTx

	stopc  chan struct{}
	startc chan struct{}
	donec  chan struct{}
}

func New(path string, d time.Duration, limit int) Backend {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		log.Panicf("backend: cannot open database at %s (%v)", path, err)
	}

	b := &backend{
		db: db,

		batchInterval: d,
		batchLimit:    limit,
		batchTx:       &batchTx{},

		stopc:  make(chan struct{}),
		startc: make(chan struct{}),
		donec:  make(chan struct{}),
	}
	b.batchTx.backend = b
	go b.run()
	<-b.startc
	return b
}

// BatchTnx returns the current batch tx in coalescer. The tx can be used for read and
// write operations. The write result can be retrieved within the same tx immediately.
// The write result is isolated with other txs until the current one get committed.
func (b *backend) BatchTx() BatchTx {
	return b.batchTx
}

// force commit the current batching tx.
func (b *backend) ForceCommit() {
	b.batchTx.Commit()
}

func (b *backend) run() {
	defer close(b.donec)

	b.batchTx.Commit()
	b.startc <- struct{}{}

	for {
		select {
		case <-time.After(b.batchInterval):
		case <-b.stopc:
			return
		}
		b.batchTx.Commit()
	}
}

func (b *backend) Close() error {
	close(b.stopc)
	<-b.donec
	return b.db.Close()
}
