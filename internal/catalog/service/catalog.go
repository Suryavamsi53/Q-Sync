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
	Row        string `json:"row,omitempty"`
	Col        int    `json:"col,omitempty"`
	Tier       string `json:"tier,omitempty"` // VIP, PLATINUM, GOLD, SILVER
	PriceCents int    `json:"price_cents"`
	Status     string `json:"status"` // AVAILABLE, HELD, SOLD
}

type Event struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Category    string     `json:"category"` // Movies, Concerts, Comedy, Sports
	Genre       string     `json:"genre"`    // e.g. "Pop / Rock • English"
	Rating      string     `json:"rating"`   // e.g. "9.8"
	Votes       string     `json:"votes"`    // e.g. "124.5k"
	Duration    string     `json:"duration"` // e.g. "2h 45m"
	Venue       string     `json:"venue"`
	City        string     `json:"city"`
	Format      string     `json:"format"`   // "IMAX 3D", "Dolby Atmos", "Live Stadium"
	Poster      string     `json:"poster"`
	Banner      string     `json:"banner"`
	PriceFrom   int        `json:"price_from"`
	Showtimes   []string   `json:"showtimes"`
	TotalSeats  int        `json:"total_seats"`
	StartTime   time.Time  `json:"start_time"`
	Seats       []SeatInfo `json:"seats,omitempty"`
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

func generateTieredSeats(prefix string, total int, basePriceCents int) []SeatInfo {
	seats := make([]SeatInfo, total)
	rows := []string{"A", "B", "C", "D", "E", "F", "G", "H", "J", "K", "L", "M"}
	colsPerRow := 20
	if total <= 120 {
		colsPerRow = 12
	} else if total <= 240 {
		colsPerRow = 16
	}

	for i := 0; i < total; i++ {
		rowIndex := (i / colsPerRow) % len(rows)
		colNumber := (i % colsPerRow) + 1
		rowLetter := rows[rowIndex]

		tier := "SILVER"
		multiplier := 1.0
		switch {
		case rowIndex <= 1:
			tier = "VIP"
			multiplier = 2.5
		case rowIndex <= 4:
			tier = "PLATINUM"
			multiplier = 1.8
		case rowIndex <= 8:
			tier = "GOLD"
			multiplier = 1.3
		default:
			tier = "SILVER"
			multiplier = 1.0
		}

		seatID := fmt.Sprintf("%s-%03d", prefix, i+1)
		// For backward compatibility with stress test and booking engine
		if prefix == "seat" {
			seatID = fmt.Sprintf("seat-%03d", i+1)
		}

		seats[i] = SeatInfo{
			ID:         seatID,
			SeatNumber: fmt.Sprintf("%s%d", rowLetter, colNumber),
			Row:        rowLetter,
			Col:        colNumber,
			Tier:       tier,
			PriceCents: int(float64(basePriceCents) * multiplier),
			Status:     "AVAILABLE",
		}
	}
	return seats
}

func (s *CatalogService) seedSampleEvents() {
	now := time.Now()

	s.events["evt-coldplay-2026"] = &Event{
		ID:         "evt-coldplay-2026",
		Name:       "Coldplay: Music of the Spheres World Tour 2026",
		Category:   "Concerts",
		Genre:      "Pop / British Rock • English",
		Rating:     "9.9",
		Votes:      "385.4K",
		Duration:   "3h 15m",
		Venue:      "DY Patil Stadium, Navi Mumbai",
		City:       "Mumbai",
		Format:     "Live Stadium Experience • Infinity Stage",
		Poster:     "https://images.unsplash.com/photo-1540039155733-5bb30b53aa14?auto=format&fit=crop&w=700&q=80",
		Banner:     "https://images.unsplash.com/photo-1470225620780-dba8ba36b745?auto=format&fit=crop&w=1600&q=80",
		PriceFrom:  2500,
		Showtimes:  []string{"06:30 PM", "08:30 PM"},
		TotalSeats: 500,
		StartTime:  now.Add(15 * 24 * time.Hour),
		Seats:      generateTieredSeats("seat", 500, 250000),
	}

	s.events["evt-diljit-2026"] = &Event{
		ID:         "evt-diljit-2026",
		Name:       "Diljit Dosanjh: DIL-LUMINATI Tour India",
		Category:   "Concerts",
		Genre:      "Punjabi Pop / Bhangra • Live Arena",
		Rating:     "9.8",
		Votes:      "294.1K",
		Duration:   "2h 50m",
		Venue:      "Jawaharlal Nehru Stadium, Delhi NCR",
		City:       "Delhi-NCR",
		Format:     "Live 360° Rotating Stage",
		Poster:     "https://images.unsplash.com/photo-1514525253161-7a46d19cd819?auto=format&fit=crop&w=700&q=80",
		Banner:     "https://images.unsplash.com/photo-1492684223066-81342ee5ff30?auto=format&fit=crop&w=1600&q=80",
		PriceFrom:  1999,
		Showtimes:  []string{"07:00 PM"},
		TotalSeats: 300,
		StartTime:  now.Add(20 * 24 * time.Hour),
		Seats:      generateTieredSeats("seat-diljit", 300, 199900),
	}

	s.events["evt-dune-2026"] = &Event{
		ID:         "evt-dune-2026",
		Name:       "Dune: Part Two (IMAX Experience + Exclusive Preview)",
		Category:   "Movies",
		Genre:      "Sci-Fi / Action Epic • English",
		Rating:     "9.6",
		Votes:      "521.8K",
		Duration:   "2h 46m",
		Venue:      "PVR INOX: Laser IMAX, Phoenix Palladium",
		City:       "Mumbai",
		Format:     "IMAX Laser 3D • Dolby Atmos 7.1.4",
		Poster:     "https://images.unsplash.com/photo-1534447677768-be436bb09401?auto=format&fit=crop&w=700&q=80",
		Banner:     "https://images.unsplash.com/photo-1518709268805-4e9042af9f23?auto=format&fit=crop&w=1600&q=80",
		PriceFrom:  450,
		Showtimes:  []string{"11:15 AM", "02:45 PM", "06:30 PM", "10:15 PM"},
		TotalSeats: 180,
		StartTime:  now.Add(2 * 24 * time.Hour),
		Seats:      generateTieredSeats("seat-dune", 180, 45000),
	}

	s.events["evt-interstellar-2026"] = &Event{
		ID:         "evt-interstellar-2026",
		Name:       "Interstellar: 10th Anniversary IMAX 70mm Special",
		Category:   "Movies",
		Genre:      "Sci-Fi / Adventure • Christopher Nolan",
		Rating:     "9.9",
		Votes:      "894.2K",
		Duration:   "2h 49m",
		Venue:      "Cinepolis: IMAX with Laser, Orion Mall",
		City:       "Bengaluru",
		Format:     "IMAX 70mm Dual Laser • 12-Track High Fidelity",
		Poster:     "https://images.unsplash.com/photo-1451187580459-43490279c0fa?auto=format&fit=crop&w=700&q=80",
		Banner:     "https://images.unsplash.com/photo-1506703719100-a0f3a48c0f86?auto=format&fit=crop&w=1600&q=80",
		PriceFrom:  550,
		Showtimes:  []string{"10:30 AM", "02:00 PM", "05:45 PM", "09:30 PM"},
		TotalSeats: 200,
		StartTime:  now.Add(3 * 24 * time.Hour),
		Seats:      generateTieredSeats("seat-interstellar", 200, 55000),
	}

	s.events["evt-ipl-final-2026"] = &Event{
		ID:         "evt-ipl-final-2026",
		Name:       "TATA IPL 2026 Grand Final: MI vs CSK",
		Category:   "Sports",
		Genre:      "T20 Cricket • World Premiere Climax",
		Rating:     "9.9",
		Votes:      "715.3K",
		Duration:   "4h 00m",
		Venue:      "Wankhede Stadium, Marine Drive",
		City:       "Mumbai",
		Format:     "Live Stadium Atmosphere • Floodlight Action",
		Poster:     "https://images.unsplash.com/photo-1540747913346-19e32dc3e97e?auto=format&fit=crop&w=700&q=80",
		Banner:     "https://images.unsplash.com/photo-1531415074868-036b107e775a?auto=format&fit=crop&w=1600&q=80",
		PriceFrom:  1500,
		Showtimes:  []string{"07:30 PM"},
		TotalSeats: 400,
		StartTime:  now.Add(10 * 24 * time.Hour),
		Seats:      generateTieredSeats("seat-ipl", 400, 150000),
	}

	s.events["evt-samay-2026"] = &Event{
		ID:         "evt-samay-2026",
		Name:       "Samay Raina: Unfiltered & Live Standup",
		Category:   "Comedy",
		Genre:      "Standup Comedy • Hinglish • 18+ Exclusive",
		Rating:     "9.7",
		Votes:      "168.5K",
		Duration:   "1h 45m",
		Venue:      "Shanmukhananda Chandrasekarendra Auditorium",
		City:       "Mumbai",
		Format:     "Auditorium Live Standup",
		Poster:     "https://images.unsplash.com/photo-1585699324551-f6c309eedeca?auto=format&fit=crop&w=700&q=80",
		Banner:     "https://images.unsplash.com/photo-1516450360452-9312f5e86fc7?auto=format&fit=crop&w=1600&q=80",
		PriceFrom:  999,
		Showtimes:  []string{"05:30 PM", "08:30 PM"},
		TotalSeats: 160,
		StartTime:  now.Add(5 * 24 * time.Hour),
		Seats:      generateTieredSeats("seat-samay", 160, 99900),
	}

	s.events["evt-arijit-2026"] = &Event{
		ID:         "evt-arijit-2026",
		Name:       "Arijit Singh: Symphony of Romance India Tour",
		Category:   "Concerts",
		Genre:      "Bollywood Romantic / Sufi • Grand Orchestra",
		Rating:     "9.8",
		Votes:      "412.9K",
		Duration:   "3h 30m",
		Venue:      "GMR Arena, Shamshabad",
		City:       "Hyderabad",
		Format:     "Grand Open Air Orchestra",
		Poster:     "https://images.unsplash.com/photo-1511671782779-c97d3d27a1d4?auto=format&fit=crop&w=700&q=80",
		Banner:     "https://images.unsplash.com/photo-1465847899084-d164df4dedc6?auto=format&fit=crop&w=1600&q=80",
		PriceFrom:  1799,
		Showtimes:  []string{"07:00 PM"},
		TotalSeats: 350,
		StartTime:  now.Add(25 * 24 * time.Hour),
		Seats:      generateTieredSeats("seat-arijit", 350, 179900),
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
			Category:   ev.Category,
			Genre:      ev.Genre,
			Rating:     ev.Rating,
			Votes:      ev.Votes,
			Duration:   ev.Duration,
			Venue:      ev.Venue,
			City:       ev.City,
			Format:     ev.Format,
			Poster:     ev.Poster,
			Banner:     ev.Banner,
			PriceFrom:  ev.PriceFrom,
			Showtimes:  ev.Showtimes,
			TotalSeats: ev.TotalSeats,
			StartTime:  ev.StartTime,
		})
	}
	return list
}
