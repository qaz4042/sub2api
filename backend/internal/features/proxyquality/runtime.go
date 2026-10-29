package proxyquality

import (
	"context"
	"errors"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Runner struct {
	store        *Store
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	observations chan observation
	active       atomic.Value
	states       sync.Map
}

func New(s *Store) *Runner {
	r := &Runner{store: s, observations: make(chan observation, 1024)}
	r.active.Store(map[int64]struct{}{})
	if s != nil {
		s.guard = r
	}
	return r
}
func (r *Runner) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(3)
	go r.runObserver(ctx)
	go r.runCleanup(ctx)
	go func() {
		defer r.wg.Done()
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				for i := 0; i < 32 && ctx.Err() == nil; i++ {
					c, stop := context.WithTimeout(ctx, 3*time.Second)
					more, err := r.store.Step(c)
					stop()
					if err != nil {
						if ctx.Err() == nil {
							log.Printf("[AccountProxyQuality] evaluation failed: %v", err)
						}
						break
					}
					if !more {
						break
					}
				}
			}
		}
	}()
}
func (r *Runner) Stop() {
	if r != nil && r.cancel != nil {
		r.cancel()
		r.wg.Wait()
	}
}

// Register only on the existing admin-authenticated proxy group.
func (r *Runner) Register(g *gin.RouterGroup) {
	g.GET("/account-auto-switch/:id", r.handle)
	g.PUT("/account-auto-switch/:id", r.handle)
}
func (r *Runner) handle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if c.Request.Method == "PUT" {
		var cfg Config
		if c.ShouldBindJSON(&cfg) != nil {
			response.BadRequest(c, "Invalid auto-switch configuration")
			return
		}
		err = r.store.Configure(ctx, id, cfg)
		if err == nil {
			response.Success(c, gin.H{"saved": true})
			return
		}
	} else {
		var settings *Settings
		settings, err = r.store.Read(ctx, id)
		if err == nil {
			response.Success(c, settings)
			return
		}
	}
	switch {
	case errors.Is(err, ErrInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, ErrConflict):
		response.Error(c, 409, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, 404, err.Error())
	default:
		response.Error(c, 500, "Account proxy auto-switch unavailable")
	}
}
