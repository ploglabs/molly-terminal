package db

import "github.com/ploglabs/molly-terminal/internal/model"

type Store struct{}

func New(_ string) (*Store, error) {
	return &Store{}, nil
}

func (s *Store) InsertMessage(_ model.Message) error {
	return nil
}

func (s *Store) GetMessages(_ string, _ int) ([]model.Message, error) {
	return nil, nil
}

func (s *Store) SearchMessages(_ string) ([]model.Message, error) {
	return nil, nil
}

func (s *Store) Close() error {
	return nil
}
