package activity

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	types []Type
	err   error
}

func (repository *fakeRepository) ListActive(ctx context.Context) ([]Type, error) {
	return repository.types, repository.err
}

func TestServiceListTypes(t *testing.T) {
	want := []Type{
		{
			Code:  "feed",
			Title: "Покормить",
		},
		{
			Code:  "play",
			Title: "Поиграть",
		},
	}

	repository := &fakeRepository{
		types: want,
	}

	service := NewService(repository, nil, nil, nil, nil)

	got, err := service.ListTypes(context.Background())
	if err != nil {
		t.Fatalf("ListTypes() error = %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("ListTypes() returned %d types, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].Code != want[i].Code {
			t.Errorf(
				"types[%d].Code = %q, want %q",
				i,
				got[i].Code,
				want[i].Code,
			)
		}
	}
}

func TestServiceListTypesRepositoryError(t *testing.T) {
	wantErr := errors.New("database error")

	repository := &fakeRepository{
		err: wantErr,
	}

	service := NewService(repository, nil, nil, nil, nil)

	_, err := service.ListTypes(context.Background())

	if !errors.Is(err, wantErr) {
		t.Errorf("ListTypes() error = %v, want %v", err, wantErr)
	}
}
