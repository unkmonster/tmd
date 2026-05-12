package api

import "testing"

func TestTaskTargetKeys_UserTask(t *testing.T) {
	task := &Task{
		Type: TaskTypeUserDownload,
		Data: &UserDownloadTaskData{ScreenName: "@Alice"},
	}
	keys := taskTargetKeys(task)
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].scope != targetScopeUser || keys[0].value != "Alice" {
		t.Fatalf("unexpected key: %+v", keys[0])
	}
}

func TestTaskTargetKeys_BatchWithListUsesWildcard(t *testing.T) {
	task := &Task{
		Type: TaskTypeBatchDownload,
		Data: &BatchDownloadTaskData{
			Users: []string{"alice"},
			Lists: []StringUint64{1},
		},
	}
	keys := taskTargetKeys(task)
	if len(keys) != 1 || keys[0] != wildcardUserTarget {
		t.Fatalf("expected wildcard key, got %+v", keys)
	}
}

func TestTaskTargetKeys_FollowingMarkUsesWildcard(t *testing.T) {
	task := &Task{
		Type: TaskTypeMarkDownloaded,
		Data: &FollowingMarkDownloadedTaskData{ScreenName: "alice"},
	}
	keys := taskTargetKeys(task)
	if len(keys) != 1 || keys[0] != wildcardUserTarget {
		t.Fatalf("expected wildcard key, got %+v", keys)
	}
}

func TestTargetsConflict(t *testing.T) {
	if !targetsConflict(targetKey{scope: targetScopeUser, value: "alice"}, wildcardUserTarget) {
		t.Fatal("expected wildcard conflict")
	}
	if !targetsConflict(targetKey{scope: targetScopeUser, value: "alice"}, targetKey{scope: targetScopeUser, value: "alice"}) {
		t.Fatal("expected same-user conflict")
	}
	if targetsConflict(targetKey{scope: targetScopeUser, value: "alice"}, targetKey{scope: targetScopeUser, value: "bob"}) {
		t.Fatal("did not expect different-user conflict")
	}
}
