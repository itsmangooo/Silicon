package updates

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type fakeUpdateStore struct {
	operation store.SystemUpdate
	statuses  []string
}

func (*fakeUpdateStore) RequeueInterruptedSystemUpdates(context.Context) error { return nil }

func (repository *fakeUpdateStore) ClaimSystemUpdate(context.Context) (store.SystemUpdate, error) {
	if repository.operation.ID == uuid.Nil {
		return store.SystemUpdate{}, store.ErrNotFound
	}
	operation := repository.operation
	repository.operation.ID = uuid.Nil
	return operation, nil
}

func (repository *fakeUpdateStore) SetSystemUpdateStatus(_ context.Context, _ uuid.UUID, status, _ string) error {
	repository.statuses = append(repository.statuses, status)
	return nil
}

type exactReleaseSource struct{ requested string }

func (*exactReleaseSource) LatestStable(context.Context) (Release, error) {
	return Release{TagName: "v0.4.2"}, nil
}
func (source *exactReleaseSource) Release(_ context.Context, tag string) (Release, error) {
	source.requested = tag
	if tag != "v0.4.2" {
		return Release{}, ErrNoRelease
	}
	return Release{TagName: tag}, nil
}

type fakeApplier struct {
	target string
	fail   bool
}

func (applier *fakeApplier) Apply(_ context.Context, target string, progress func(string, string) error) error {
	applier.target = target
	if applier.fail {
		return errors.New("preparation failed")
	}
	for _, status := range []string{"preparing", "updating", "migrating", "restarting", "waiting_for_health"} {
		if err := progress(status, status); err != nil {
			return err
		}
	}
	return nil
}

func TestRunnerAppliesExactReleaseAndPersistsProgress(t *testing.T) {
	repository := &fakeUpdateStore{operation: store.SystemUpdate{ID: uuid.New(), FromVersion: "v0.4.1", TargetVersion: "v0.4.2", Status: "checking"}}
	releases := &exactReleaseSource{}
	applier := &fakeApplier{}
	runner := Runner{Repository: repository, Releases: releases, Applier: applier}
	processed, err := runner.ProcessOne(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessOne processed=%v err=%v", processed, err)
	}
	if releases.requested != "v0.4.2" || applier.target != "v0.4.2" {
		t.Fatalf("exact target was not preserved: release=%q apply=%q", releases.requested, applier.target)
	}
	want := []string{"preparing", "updating", "migrating", "restarting", "waiting_for_health", "completed"}
	if len(repository.statuses) != len(want) {
		t.Fatalf("statuses=%v want=%v", repository.statuses, want)
	}
	for index := range want {
		if repository.statuses[index] != want[index] {
			t.Fatalf("statuses=%v want=%v", repository.statuses, want)
		}
	}
}

func TestRunnerPersistsFailureWithoutApplyingUnverifiedRelease(t *testing.T) {
	repository := &fakeUpdateStore{operation: store.SystemUpdate{ID: uuid.New(), FromVersion: "v0.4.1", TargetVersion: "v0.4.3", Status: "checking"}}
	applier := &fakeApplier{}
	runner := Runner{Repository: repository, Releases: &exactReleaseSource{}, Applier: applier}
	processed, err := runner.ProcessOne(context.Background())
	if !processed || err == nil {
		t.Fatalf("ProcessOne processed=%v err=%v", processed, err)
	}
	if applier.target != "" {
		t.Fatalf("unverified target reached installer: %q", applier.target)
	}
	if len(repository.statuses) != 1 || repository.statuses[0] != "failed" {
		t.Fatalf("failure status not persisted: %v", repository.statuses)
	}
}
