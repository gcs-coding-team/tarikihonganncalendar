package service

import (
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type ColonyService struct {
	repo repository.ColonyRepository
}

type CreateColonyInput struct {
	Name        string
	Description string
}

type UpdateColonyInput struct {
	Name        *string
	Description *string
}

type CreateSharedItemInput struct {
	SourceType string
	SourceID   string
}

func NewColonyService(repo repository.ColonyRepository) *ColonyService {
	return &ColonyService{repo: repo}
}

func (s *ColonyService) Create(userID string, input CreateColonyInput) (repository.Colony, error) {
	if userID == "" {
		return repository.Colony{}, repository.ErrForbidden
	}
	colony := repository.Colony{
		Name:        input.Name,
		Description: input.Description,
		OwnerUserID: userID,
	}
	return s.repo.CreateColony(colony)
}

func (s *ColonyService) List(userID string) ([]repository.Colony, error) {
	return s.repo.ListColonies(userID)
}

func (s *ColonyService) Get(userID, colonyID string) (repository.Colony, error) {
	return s.repo.GetColony(userID, colonyID)
}

func (s *ColonyService) Update(userID, colonyID string, input UpdateColonyInput) (repository.Colony, error) {
	colony, err := s.repo.GetColony(userID, colonyID)
	if err != nil {
		return repository.Colony{}, err
	}
	if input.Name != nil {
		colony.Name = *input.Name
	}
	if input.Description != nil {
		colony.Description = *input.Description
	}
	return s.repo.UpdateColony(colony)
}

func (s *ColonyService) Delete(userID, colonyID string) error {
	return s.repo.DeleteColony(userID, colonyID)
}

func (s *ColonyService) Join(userID, colonyID, inviteCode string) (repository.Colony, error) {
	return s.repo.JoinColony(userID, colonyID, inviteCode)
}

func (s *ColonyService) Leave(userID, colonyID string) error {
	return s.repo.LeaveColony(userID, colonyID)
}

func (s *ColonyService) ListMembers(colonyID string) ([]repository.ColonyMember, error) {
	return s.repo.ListColonyMembers(colonyID)
}

func (s *ColonyService) CreateSharedItem(userID, colonyID string, input CreateSharedItemInput) (repository.SharedItem, error) {
	_, err := s.repo.GetColony(userID, colonyID)
	if err != nil {
		return repository.SharedItem{}, err
	}
	item := repository.SharedItem{
		ColonyID:   colonyID,
		SourceType: input.SourceType,
		SourceID:   input.SourceID,
		CreatedBy:  userID,
	}
	return s.repo.CreateSharedItem(item)
}

func (s *ColonyService) DeleteSharedItem(userID, colonyID, sharedItemID string) error {
	return s.repo.DeleteSharedItem(userID, colonyID, sharedItemID)
}

func (s *ColonyService) Feed(colonyID string) ([]repository.SharedItem, error) {
	return s.repo.ListSharedItems(colonyID)
}
