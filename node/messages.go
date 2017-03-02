package node

import (
	"time"

	"github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/daemon/gnet"

	"github.com/skycoin/cxo/data"
)

type Ping struct{}

func (p *Ping) Handle(ctx *gnet.MessageContext, _ interface{}) (_ error) {
	ctx.Conn.ConnectionPool.SendMessage(ctx.Conn, &Pong{})
	return
}

type Pong struct{}

func (*Pong) Handle(_ *gnet.MessageContext, _ interface{}) (_ error) {
	return
}

type Announce struct {
	Hash cipher.SHA256
}

func (a *Announce) Handle(ctx *gnet.MessageContext,
	node interface{}) (terminate error) {

	n := node.(*Node)
	if n.db.Has(a.Hash) {
		return
	}
	ctx.Conn.ConnectionPool.SendMessage(ctx.Conn, &Request{a.Hash})
	return
}

type Request struct {
	Hash cipher.SHA256
}

func (r *Request) Handle(ctx *gnet.MessageContext,
	node interface{}) (terminate error) {

	n := node.(*Node)
	if data, ok := n.db.Get(a.Hash); ok {
		ctx.Conn.ConnectionPool.SendMessage(ctx.Conn, &Data{data})
	}
	return
}

type Data struct {
	Data []byte
}

func (d *Data) Handle(ctx *gnet.MessageContext,
	node interface{}) (terminate error) {

	// TODO: drop data I don't asking for

	n := node.(*Node)
	hash := cipher.SumSHA256(d.Data)
	n.db.Set(hash, d.Data)
	// Broadcast
	for _, gc := range n.pool.Pool {
		if gc != ctx.Conn {
			n.pool.SendMessage(gc, Announce{hash})
		}
	}
	return
}
