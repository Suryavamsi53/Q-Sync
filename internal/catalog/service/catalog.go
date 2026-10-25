package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var ErrEventNotFound = errors.New("event not found")

type SeatInfo struct {
	ID         string `json:"id"`
	SeatNumber string `json:"seat_number"`
	PriceCents int    `json:"price_cents"`
	Status     string `json:"status"` // AVAILABLE, HELD, SOLD
}

type Event struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Venue      string     `json:"venue"`
	TotalSeats int        `json:"total_seats"`
	StartTime  time.Time  `json:"start_time"`
	Seats      []SeatInfo `json:"seats,omitempty"`
}

type CatalogService struct {
	mu     sync.RWMutex
	events map[string]*Event
	group  singleflight.Group
}

func NewCatalogService() *CatalogService {
	svc := &CatalogService{
		events: make(map[string]*Event),
	}
	svc.seedSampleEvents()
	return svc
}

func (s *CatalogService) seedSampleEvents() {
	eventID := "evt-coldplay-2026"
	seats := make([]SeatInfo, 500)
	for i := 1; i <= 500; i++ {
		seats[i-1] = SeatInfo{
			ID:         fmt.Sprintf("seat-%03d", i),
			SeatNumber: fmt.Sprintf("Seat %d", i),
			PriceCents: 15000,
			Status:     "AVAILABLE",
		}
	}

	s.events[eventID] = &Event{
		ID:         eventID,
		Name:       "Coldplay Music of the Spheres Tour 2026",
		Venue:      "Wembley Stadium, London",
		TotalSeats: 500,
		StartTime:  time.Now().Add(30 * 24 * time.Hour),
		Seats:      seats,
	}
}

// GetEventWithSingleflight prevents cache stampede / thundering herd under 20,000 concurrent reads
func (s *CatalogService) GetEventWithSingleflight(ctx context.Context, eventID string) (*Event, error) {
	// Deduplicate concurrent requests for the same eventID into a single execution
	result, err, shared := s.group.Do(eventID, func() (any, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		ev, exists := s.events[eventID]
		if !exists {
			return nil, ErrEventNotFound
		}

		// Simulate slight DB fetch latency (10ms)
		time.Sleep(10 * time.Millisecond)

		// Return deep copy
		data, _ := json.Marshal(ev)
		var copyEv Event
		_ = json.Unmarshal(data, &copyEv)
		return &copyEv, nil
	})

	if err != nil {
		return nil, err
	}

	_ = shared // in production: emit metric singleflight_shared_requests_total
	return result.(*Event), nil
}

func (s *CatalogService) ListEvents(ctx context.Context) []*Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Event, 0, len(s.events))
	for _, ev := range s.events {
		list = append(list, &Event{
			ID:         ev.ID,
			Name:       ev.Name,
			Venue:      ev.Venue,
			TotalSeats: ev.TotalSeats,
			StartTime:  ev.StartTime,
		})
	}
	return list
}
