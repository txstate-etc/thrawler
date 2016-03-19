// Hashed PROCesseS (hprocs)
package main

import (
	log "gopkg.in/inconshreveable/log15.v2"
	"hash/fnv"
	"sync"
)

type ProcInfo interface {
	String() string
	Fn(log.Logger, int) []ProcInfo
}

type proc chan ProcInfo

type procs struct {
	wg    *sync.WaitGroup
	chans []proc
}

// 1) Pull off from corresponding channel
// 2) Process request
// 3) Remove from WaitGroup
func (ps *procs) listen(l log.Logger, i int) {
	l = l.New("thd", i)
	for pi := range ps.chans[i] {
		ps.spawnFill(pi.Fn(l, i))
		ps.wg.Done()
	}
}

// 1) Add to WaitGroup
// 2) Kick off process to fill channels
func (ps *procs) spawnFill(pis []ProcInfo) {
	if len(pis) > 0 {
		ps.wg.Add(len(pis))
		go ps.fill(pis)
	}
}

// TODO: If we end up blocking on a channel
// we can convert this to non-blocking by
// utilizing a select and rotating through
// the ProcInfo List to try other channels.
func (ps *procs) fill(pis []ProcInfo) {
	for len(pis) > 0 {
		h := fnv.New64()
		h.Write([]byte(pis[0].String()))
		i := int(h.Sum64() % uint64(len(ps.chans)))
		ps.chans[i] <- pis[0]
		pis = pis[1:]
	}
}

func Run(l log.Logger, num int, pis []ProcInfo) {
	chans := make([]proc, num)
	var wg sync.WaitGroup
	ps := procs{wg: &wg, chans: chans}
	for i := range chans {
		chans[i] = make(proc, num)
		go ps.listen(l, i)
	}
	ps.spawnFill(pis)
	wg.Wait()
	for _, ch := range chans {
		close(ch)
	}
}
