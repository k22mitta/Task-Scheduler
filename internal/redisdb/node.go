package redisdb

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Node struct {
	id     string
	client *redis.Client
}

func NewNode(client *redis.Client) *Node {
	return &Node{
		id:     uuid.New().String(),
		client: client,
	}
}

func (n *Node) Register(ctx context.Context) error {
	return n.client.Set(ctx, "node:"+n.id, "alive", 30*time.Second).Err()
}

func (n *Node) Heartbeat(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := n.Register(ctx); err != nil {
				log.Printf("node %s: heartbeat failed: %v", n.id, err)
			}
		}
	}
}

func (n *Node) ID() string {
	return n.id
}
