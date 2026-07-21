package domain

import "time"

type TimetableEntry struct {
	ID        string
	UserID    string
	DayOfWeek int16
	Period    int16
	Subject   string
	Room      string
	Teacher   string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}
