package repository

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrDuplicate = errors.New("duplicate")
	ErrForbidden = errors.New("forbidden")

	errValidation = errors.New("validation error")
)

func ValidationError(msg string) error {
	return fmt.Errorf("%w: %s", errValidation, msg)
}

func IsValidationError(err error) bool {
	return errors.Is(err, errValidation)
}

// Repeat describes a recurring event. Freq is daily, weekly or monthly; the
// series starts on the event's StartAt and runs until Until, or forever when
// Until is empty. Dates are held as YYYY-MM-DD because a recurrence rule is
// about calendar days, not instants.
type Repeat struct {
	Freq  string `json:"freq"`
	Until string `json:"until,omitempty"`
}

type Event struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartAt     time.Time `json:"startAt"`
	EndAt       time.Time `json:"endAt"`
	AllDay      bool      `json:"allDay"`
	// Repeat is nil for a one-off event. Expanding the series is left to the
	// caller; what is stored here is the rule and the days dropped from it.
	Repeat  *Repeat  `json:"repeat,omitempty"`
	ExDates []string `json:"exdates,omitempty"`

	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TimetableEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	DayOfWeek int       `json:"dayOfWeek"`
	Period    int       `json:"period"`
	Subject   string    `json:"subject"`
	Room      string    `json:"room"`
	Teacher   string    `json:"teacher"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Colony struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerUserID string    `json:"ownerUserId"`
	InviteCode  string    `json:"inviteCode,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ColonyMember struct {
	ColonyID string `json:"colonyId"`
	UserID   string `json:"userId"`
	// DisplayName is filled in from the user record. Without it a colony's
	// member list is a column of opaque ids, which tells nobody who is in it.
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joinedAt"`
}

type SharedItem struct {
	ID            string     `json:"id"`
	ColonyID      string     `json:"colonyId"`
	SourceType    string     `json:"sourceType"`
	SourceID      string     `json:"sourceId"`
	CreatedBy     string     `json:"createdBy"`
	TitleSnapshot string     `json:"titleSnapshot"`
	DateSnapshot  *time.Time `json:"dateSnapshot,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type EventRepository interface {
	CreateEvent(event Event) (Event, error)
	ListEvents(userID, cursor string, limit int) ([]Event, error)
	GetEvent(userID, eventID string) (Event, error)
	UpdateEvent(event Event) (Event, error)
	DeleteEvent(userID, eventID string) error
}

type TimetableRepository interface {
	CreateTimetableEntry(entry TimetableEntry) (TimetableEntry, error)
	ListTimetableEntries(userID string) ([]TimetableEntry, error)
	GetTimetableEntry(userID, entryID string) (TimetableEntry, error)
	UpdateTimetableEntry(entry TimetableEntry) (TimetableEntry, error)
	DeleteTimetableEntry(userID, entryID string) error
}

type ColonyRepository interface {
	CreateColony(colony Colony) (Colony, error)
	ListColonies(userID string) ([]Colony, error)
	GetColony(userID, colonyID string) (Colony, error)
	UpdateColony(colony Colony) (Colony, error)
	DeleteColony(userID, colonyID string) error
	FindColonyByInviteCode(inviteCode string) (Colony, error)
	JoinColony(userID, colonyID, inviteCode string) (Colony, error)
	LeaveColony(userID, colonyID string) error
	ListColonyMembers(colonyID string) ([]ColonyMember, error)
	CreateSharedItem(item SharedItem) (SharedItem, error)
	ListSharedItems(colonyID string) ([]SharedItem, error)
	DeleteSharedItem(userID, colonyID, sharedItemID string) error
}

type Repository interface {
	EventRepository
	TimetableRepository
	ColonyRepository
	TaskRepository
	ProjectRepository
	UserRepository
	CandidateRepository
	PasswordResetRepository
	PrintRepository
	SessionRepository
	AnalysisJobRepository
}

type Session struct {
	ID        string
	UserID    string
	Token     string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AnalysisJob struct {
	ID            string
	UserID        string
	ContentType   string
	Filename      string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ResultSummary string
}

type SessionRepository interface {
	CreateSession(session Session) (Session, error)
	GetSessionByToken(token string) (Session, error)
	DeleteSession(token string) error
}

type AnalysisJobRepository interface {
	CreateAnalysisJob(job AnalysisJob) (AnalysisJob, error)
	GetAnalysisJob(jobID string) (AnalysisJob, error)
	ListAnalysisJobs(userID string) ([]AnalysisJob, error)
	UpdateAnalysisJob(job AnalysisJob) (AnalysisJob, error)
}

type MemoryRepository struct {
	mu            sync.RWMutex
	events        map[string]Event
	timetable     map[string]TimetableEntry
	colonies      map[string]Colony
	colonyMembers map[string]map[string]ColonyMember
	sharedItems   map[string]SharedItem
	colonyIndex   map[string][]string
	sessions      map[string]Session
	analysisJobs  map[string]AnalysisJob
	taskStore
	userStore
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		events:        make(map[string]Event),
		timetable:     make(map[string]TimetableEntry),
		colonies:      make(map[string]Colony),
		colonyMembers: make(map[string]map[string]ColonyMember),
		sharedItems:   make(map[string]SharedItem),
		colonyIndex:   make(map[string][]string),
		sessions:      make(map[string]Session),
		analysisJobs:  make(map[string]AnalysisJob),
		taskStore:     newTaskStore(),
		userStore:     newUserStore(),
	}
}

func (r *MemoryRepository) CreateEvent(event Event) (Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.ID == "" {
		event.ID = newID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = event.CreatedAt
	}
	if event.Version == 0 {
		event.Version = 1
	}
	r.events[event.ID] = event
	return event, nil
}

func (r *MemoryRepository) ListEvents(userID, cursor string, limit int) ([]Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	items := make([]Event, 0, limit)
	for _, event := range r.events {
		if event.UserID != userID {
			continue
		}
		if cursor != "" && event.ID == cursor {
			continue
		}
		items = append(items, event)
	}
	sortEvents(items)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *MemoryRepository) GetEvent(userID, eventID string) (Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	event, ok := r.events[eventID]
	if !ok || event.UserID != userID {
		return Event{}, ErrNotFound
	}
	return event, nil
}

func (r *MemoryRepository) UpdateEvent(event Event) (Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.events[event.ID]
	if !ok {
		return Event{}, ErrNotFound
	}
	if existing.Version != event.Version {
		return Event{}, ErrConflict
	}
	event.Version = existing.Version + 1
	event.UpdatedAt = time.Now().UTC()
	r.events[event.ID] = event
	return event, nil
}

func (r *MemoryRepository) DeleteEvent(userID, eventID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event, ok := r.events[eventID]
	if !ok || event.UserID != userID {
		return ErrNotFound
	}
	delete(r.events, eventID)
	return nil
}

func (r *MemoryRepository) CreateTimetableEntry(entry TimetableEntry) (TimetableEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry.ID == "" {
		entry.ID = newID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = entry.CreatedAt
	}
	if entry.Version == 0 {
		entry.Version = 1
	}
	r.timetable[entry.ID] = entry
	return entry, nil
}

func (r *MemoryRepository) ListTimetableEntries(userID string) ([]TimetableEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]TimetableEntry, 0)
	for _, entry := range r.timetable {
		if entry.UserID == userID {
			items = append(items, entry)
		}
	}
	sortTimetable(items)
	return items, nil
}

func (r *MemoryRepository) GetTimetableEntry(userID, entryID string) (TimetableEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.timetable[entryID]
	if !ok || entry.UserID != userID {
		return TimetableEntry{}, ErrNotFound
	}
	return entry, nil
}

func (r *MemoryRepository) UpdateTimetableEntry(entry TimetableEntry) (TimetableEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.timetable[entry.ID]
	if !ok {
		return TimetableEntry{}, ErrNotFound
	}
	if existing.Version != entry.Version {
		return TimetableEntry{}, ErrConflict
	}
	entry.Version = existing.Version + 1
	entry.UpdatedAt = time.Now().UTC()
	r.timetable[entry.ID] = entry
	return entry, nil
}

func (r *MemoryRepository) DeleteTimetableEntry(userID, entryID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.timetable[entryID]
	if !ok || entry.UserID != userID {
		return ErrNotFound
	}
	delete(r.timetable, entryID)
	return nil
}

func (r *MemoryRepository) CreateColony(colony Colony) (Colony, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if colony.ID == "" {
		colony.ID = newID()
	}
	if colony.CreatedAt.IsZero() {
		colony.CreatedAt = time.Now().UTC()
	}
	if colony.UpdatedAt.IsZero() {
		colony.UpdatedAt = colony.CreatedAt
	}
	if colony.InviteCode == "" {
		colony.InviteCode = fmt.Sprintf("%08d", len(r.colonies)+1)
	}
	r.colonies[colony.ID] = colony
	if _, ok := r.colonyMembers[colony.ID]; !ok {
		r.colonyMembers[colony.ID] = make(map[string]ColonyMember)
	}
	r.colonyMembers[colony.ID][colony.OwnerUserID] = ColonyMember{ColonyID: colony.ID, UserID: colony.OwnerUserID, Role: "OWNER", JoinedAt: time.Now().UTC()}
	r.colonyIndex[colony.OwnerUserID] = append(r.colonyIndex[colony.OwnerUserID], colony.ID)
	return colony, nil
}

// ListColonies returns every colony the user belongs to, not only the ones
// they created — a colony you joined is one you need to see. The creator is
// recorded as a member at creation time, so they are covered by the same check.
func (r *MemoryRepository) ListColonies(userID string) ([]Colony, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]Colony, 0)
	for _, colony := range r.colonies {
		if _, ok := r.colonyMembers[colony.ID][userID]; ok {
			items = append(items, colony)
		}
	}
	sortColonies(items)
	return items, nil
}

// FindColonyByInviteCode lets someone join with nothing but the code they were
// given. Join needs a colony ID, which an outsider has no way to discover.
func (r *MemoryRepository) FindColonyByInviteCode(inviteCode string) (Colony, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, colony := range r.colonies {
		if colony.InviteCode == inviteCode {
			return colony, nil
		}
	}
	return Colony{}, ErrNotFound
}

func (r *MemoryRepository) GetColony(userID, colonyID string) (Colony, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	colony, ok := r.colonies[colonyID]
	if !ok {
		return Colony{}, ErrNotFound
	}
	if colony.OwnerUserID != userID {
		if _, ok := r.colonyMembers[colonyID][userID]; !ok {
			return Colony{}, ErrForbidden
		}
	}
	return colony, nil
}

func (r *MemoryRepository) UpdateColony(colony Colony) (Colony, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.colonies[colony.ID]
	if !ok {
		return Colony{}, ErrNotFound
	}
	existing.Name = colony.Name
	existing.Description = colony.Description
	existing.UpdatedAt = time.Now().UTC()
	r.colonies[colony.ID] = existing
	return existing, nil
}

func (r *MemoryRepository) DeleteColony(userID, colonyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	colony, ok := r.colonies[colonyID]
	if !ok || colony.OwnerUserID != userID {
		return ErrForbidden
	}
	delete(r.colonies, colonyID)
	delete(r.colonyMembers, colonyID)
	r.deleteSharedItemsForColonyLocked(colonyID)
	return nil
}

// deleteSharedItemsForColonyLocked removes every shared item belonging to a
// colony. sharedItems is keyed by the item's own ID, not the colony's, so this
// has to scan rather than delete by colonyID directly. Callers must already
// hold r.mu.
func (r *MemoryRepository) deleteSharedItemsForColonyLocked(colonyID string) {
	for id, item := range r.sharedItems {
		if item.ColonyID == colonyID {
			delete(r.sharedItems, id)
		}
	}
}

func (r *MemoryRepository) JoinColony(userID, colonyID, inviteCode string) (Colony, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	colony, ok := r.colonies[colonyID]
	if !ok {
		return Colony{}, ErrNotFound
	}
	if colony.InviteCode != inviteCode {
		return Colony{}, ErrForbidden
	}
	if _, ok := r.colonyMembers[colonyID][userID]; !ok {
		if _, ok := r.colonyMembers[colonyID]; !ok {
			r.colonyMembers[colonyID] = make(map[string]ColonyMember)
		}
		r.colonyMembers[colonyID][userID] = ColonyMember{ColonyID: colonyID, UserID: userID, Role: "MEMBER", JoinedAt: time.Now().UTC()}
	}
	return colony, nil
}

func (r *MemoryRepository) LeaveColony(userID, colonyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.colonyMembers[colonyID][userID]; !ok {
		return ErrNotFound
	}
	delete(r.colonyMembers[colonyID], userID)
	return nil
}

func (r *MemoryRepository) ListColonyMembers(colonyID string) ([]ColonyMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	members := make([]ColonyMember, 0)
	for _, member := range r.colonyMembers[colonyID] {
		members = append(members, member)
	}
	sortMembers(members)
	// Names come from the user records. Someone who signed in through the
	// development shortcut has no record, so their id stands in.
	for i := range members {
		if u, ok := r.users[members[i].UserID]; ok && u.DisplayName != "" {
			members[i].DisplayName = u.DisplayName
		} else {
			members[i].DisplayName = members[i].UserID
		}
	}
	return members, nil
}

func (r *MemoryRepository) CreateSharedItem(item SharedItem) (SharedItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.sharedItems {
		if existing.ColonyID == item.ColonyID && existing.SourceType == item.SourceType && existing.SourceID == item.SourceID {
			return SharedItem{}, ErrDuplicate
		}
	}
	if item.ID == "" {
		item.ID = newID()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	r.sharedItems[item.ID] = item
	return item, nil
}

func (r *MemoryRepository) ListSharedItems(colonyID string) ([]SharedItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]SharedItem, 0)
	for _, item := range r.sharedItems {
		if item.ColonyID == colonyID {
			items = append(items, item)
		}
	}
	sortSharedItems(items)
	return items, nil
}

func (r *MemoryRepository) DeleteSharedItem(userID, colonyID, sharedItemID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.sharedItems[sharedItemID]
	if !ok || item.ColonyID != colonyID {
		return ErrNotFound
	}
	if item.CreatedBy != userID {
		return ErrForbidden
	}
	delete(r.sharedItems, sharedItemID)
	return nil
}

func (r *MemoryRepository) CreateSession(session Session) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session.ID == "" {
		session.ID = newID()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = session.CreatedAt
	}
	if session.Token == "" {
		session.Token = newID()
	}
	r.sessions[session.Token] = session
	return session, nil
}

func (r *MemoryRepository) GetSessionByToken(token string) (Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[token]
	if !ok {
		return Session{}, ErrNotFound
	}
	return session, nil
}

func (r *MemoryRepository) DeleteSession(token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[token]; !ok {
		return ErrNotFound
	}
	delete(r.sessions, token)
	return nil
}

func (r *MemoryRepository) CreateAnalysisJob(job AnalysisJob) (AnalysisJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job.ID == "" {
		job.ID = newID()
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = job.CreatedAt
	}
	r.analysisJobs[job.ID] = job
	return job, nil
}

func (r *MemoryRepository) GetAnalysisJob(jobID string) (AnalysisJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.analysisJobs[jobID]
	if !ok {
		return AnalysisJob{}, ErrNotFound
	}
	return job, nil
}

func (r *MemoryRepository) ListAnalysisJobs(userID string) ([]AnalysisJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]AnalysisJob, 0)
	for _, job := range r.analysisJobs {
		if job.UserID == userID {
			items = append(items, job)
		}
	}
	return items, nil
}

func (r *MemoryRepository) UpdateAnalysisJob(job AnalysisJob) (AnalysisJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = time.Now().UTC()
	}
	r.analysisJobs[job.ID] = job
	return job, nil
}

func newID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func sortEvents(items []Event) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].StartAt.After(items[j].StartAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func sortTimetable(items []TimetableEntry) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].DayOfWeek > items[j].DayOfWeek || (items[i].DayOfWeek == items[j].DayOfWeek && items[i].Period > items[j].Period) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func sortColonies(items []Colony) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].CreatedAt.After(items[j].CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func sortMembers(items []ColonyMember) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].JoinedAt.After(items[j].JoinedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func sortSharedItems(items []SharedItem) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].CreatedAt.After(items[j].CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// NewID hands out an identifier in the same format the in-memory store uses, so
// alternative backings (see repository/pgstore) mint ids the same way.
func NewID() string { return newID() }
