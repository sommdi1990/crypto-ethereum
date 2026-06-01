package blockchain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Block stores processed block metadata for reorg detection.
type Block struct {
	ID          uuid.UUID
	ChainID     int64
	BlockNumber int64
	BlockHash   string
	ParentHash  string
	ProcessedAt time.Time
}

// EventStatus tracks event processing state.
type EventStatus string

const (
	EventProcessed EventStatus = "PROCESSED"
	EventOrphaned  EventStatus = "ORPHANED"
)

// Event is a deduplicated on-chain log event.
type Event struct {
	ID              uuid.UUID
	ChainID         int64
	BlockNumber     int64
	BlockHash       string
	TxHash          string
	LogIndex        int
	ContractAddress string
	EventName       string
	Payload         map[string]interface{}
	Status          EventStatus
	ProcessedAt     time.Time
}

// DedupKey returns the unique key for event deduplication.
func DedupKey(chainID int64, txHash string, logIndex int) string {
	return fmtKey(chainID, txHash, logIndex)
}

func fmtKey(chainID int64, txHash string, logIndex int) string {
	return fmt.Sprintf("%d:%s:%d", chainID, txHash, logIndex)
}
