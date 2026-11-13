package handler

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type tableTestRow struct {
	ID      int64
	Name    string
	AdminID int32
}

type tableTestParam struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	AdminID int32  `json:"admin_id"`
}

// tableTestRows 是可观测的假仓库：记录每次调用参数，供断言 handler 的分派行为。
type tableTestRows struct {
	row tableTestRow

	gotOneID     int64
	gotOneCalled bool
	added        []tableTestRow
	edited       []tableTestRow
	deleted      any
	delCalled    bool
	updateCalled bool
	updatedField string
	updatedValue any

	// mutateErr 由 Add/Edit/Del 返回，用于覆盖失败分支。
	mutateErr error
}

func (f *tableTestRows) Table() string          { return "table_test_rows" }
func (f *tableTestRows) PrimaryKeyName() string { return "id" }

func (f *tableTestRows) GetOneScoped(*gin.Context, any) (any, error) { return f.row, nil }

func (f *tableTestRows) UpdateFieldScoped(_ *gin.Context, _ any, field string, value any) (int64, bool, error) {
	f.updateCalled = true
	f.updatedField = field
	f.updatedValue = value
	return 1, true, nil
}

func (f *tableTestRows) SortWeighScoped(*gin.Context, string, string, string, string) error {
	return nil
}

func (f *tableTestRows) GetOne(_ *gin.Context, id int64) (tableTestRow, error) {
	f.gotOneCalled = true
	f.gotOneID = id
	return f.row, nil
}

func (f *tableTestRows) List(*gin.Context) ([]tableTestRow, int64, error) { return nil, 0, nil }

func (f *tableTestRows) Add(_ *gin.Context, row tableTestRow) error {
	if f.mutateErr != nil {
		return f.mutateErr
	}
	f.added = append(f.added, row)
	return nil
}

func (f *tableTestRows) Edit(_ *gin.Context, row tableTestRow) error {
	if f.mutateErr != nil {
		return f.mutateErr
	}
	f.edited = append(f.edited, row)
	return nil
}

func (f *tableTestRows) Del(_ *gin.Context, ids any) error {
	if f.mutateErr != nil {
		return f.mutateErr
	}
	f.delCalled = true
	f.deleted = ids
	return nil
}

// newTableTestHandler 构造带 hook 计数与可选归属恢复的 TableHandler。
// restore 为真时模拟生成 header 的 reassignable 形态。
func newTableTestHandler(rows *tableTestRows, partial map[string]bool, restore bool) (*TableHandler[tableTestRow, tableTestParam, int64], *int) {
	hookCalls := 0
	opts := TableOptions[tableTestRow, tableTestParam]{
		PKJSONName:  "id",
		PartialEdit: partial,
		AfterMutation: func(*gin.Context) {
			hookCalls++
		},
	}
	if restore {
		opts.RestoreAfterCopy = func(params tableTestParam, row *tableTestRow, original tableTestRow) {
			if params.AdminID == 0 {
				row.AdminID = original.AdminID
			}
		}
	}
	h := NewTableHandler[tableTestRow, tableTestParam, int64](zap.NewNop(), rows, opts)
	return &h, &hookCalls
}

func newTableTestContext(method, target, body string) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	return ctx
}

func TestParsePrimaryKey(t *testing.T) {
	t.Run("int64", func(t *testing.T) {
		id, err := parsePrimaryKey[int64]("42")
		if err != nil || id != 42 {
			t.Fatalf("parsePrimaryKey[int64](42) = %v, %v", id, err)
		}
		for _, value := range []string{"", "abc", "0", "-1.5"} {
			if _, err := parsePrimaryKey[int64](value); err == nil {
				t.Fatalf("parsePrimaryKey[int64](%q) should fail", value)
			}
		}
	})

	t.Run("string", func(t *testing.T) {
		id, err := parsePrimaryKey[string]("abc")
		if err != nil || id != "abc" {
			t.Fatalf("parsePrimaryKey[string](abc) = %v, %v", id, err)
		}
		if id, err := parsePrimaryKey[string](`"abc"`); err != nil || id != "abc" {
			t.Fatalf("quoted json string should also parse, got %v, %v", id, err)
		}
		// 字符串主键的 "0" 是合法值（与整型主键的 0 不同），只拒绝空串。
		if id, err := parsePrimaryKey[string]("0"); err != nil || id != "0" {
			t.Fatalf(`parsePrimaryKey[string]("0") = %v, %v`, id, err)
		}
		if _, err := parsePrimaryKey[string](""); err == nil {
			t.Fatal(`parsePrimaryKey[string]("") should fail`)
		}
	})
}

// Edit 的归属恢复契约：未传 / 传 0 时保留原归属，显式传值才采纳。
func TestTableHandlerEditRestoresOwner(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantAdminID  int32
		wantName     string
		wantEditedOk bool
	}{
		{name: "未传归属保留原值", body: `{"id":1,"name":"x"}`, wantAdminID: 7, wantName: "x", wantEditedOk: true},
		{name: "归属传 0 保留原值", body: `{"id":1,"name":"y","admin_id":0}`, wantAdminID: 7, wantName: "y", wantEditedOk: true},
		{name: "显式归属采纳新值", body: `{"id":1,"name":"z","admin_id":3}`, wantAdminID: 3, wantName: "z", wantEditedOk: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := &tableTestRows{row: tableTestRow{ID: 1, Name: "old", AdminID: 7}}
			h, _ := newTableTestHandler(rows, nil, true)

			h.Edit(newTableTestContext("POST", "/edit", tc.body))

			if len(rows.edited) != 1 {
				t.Fatalf("rows.Edit called %d times, want 1", len(rows.edited))
			}
			edited := rows.edited[0]
			if edited.AdminID != tc.wantAdminID {
				t.Fatalf("edited admin id = %d, want %d", edited.AdminID, tc.wantAdminID)
			}
			if edited.Name != tc.wantName {
				t.Fatalf("edited name = %q, want %q", edited.Name, tc.wantName)
			}
			if edited.ID != 1 {
				t.Fatalf("edited id = %d, want 1", edited.ID)
			}
		})
	}
}

// 缺主键或主键为 0 时必须拒绝写入。
func TestTableHandlerEditRejectsInvalidPrimaryKey(t *testing.T) {
	for _, body := range []string{`{"name":"x"}`, `{"id":0,"name":"x"}`} {
		rows := &tableTestRows{row: tableTestRow{ID: 1, Name: "old", AdminID: 7}}
		h, hookCalls := newTableTestHandler(rows, nil, true)

		h.Edit(newTableTestContext("POST", "/edit", body))

		if len(rows.edited) != 0 {
			t.Fatalf("body %q should not reach rows.Edit", body)
		}
		if *hookCalls != 0 {
			t.Fatalf("body %q should not run AfterMutation", body)
		}
	}
}

// Edit 把解析出的主键透传给 GetOne。
func TestTableHandlerEditPassesPrimaryKey(t *testing.T) {
	rows := &tableTestRows{row: tableTestRow{ID: 42, Name: "old", AdminID: 7}}
	h, _ := newTableTestHandler(rows, nil, true)

	h.Edit(newTableTestContext("POST", "/edit", `{"id":42,"name":"x"}`))

	if !rows.gotOneCalled || rows.gotOneID != 42 {
		t.Fatalf("GetOne received id = %d (called=%v), want 42", rows.gotOneID, rows.gotOneCalled)
	}
	if len(rows.edited) != 1 || rows.edited[0].ID != 42 {
		t.Fatalf("edited rows = %+v, want one row with id 42", rows.edited)
	}
}

// 写库失败时不得执行 AfterMutation（缓存失效只属于成功响应）。
func TestTableHandlerEditFailureSkipsHook(t *testing.T) {
	rows := &tableTestRows{
		row:       tableTestRow{ID: 1, Name: "old", AdminID: 7},
		mutateErr: errors.New("edit failed"),
	}
	h, hookCalls := newTableTestHandler(rows, nil, true)

	h.Edit(newTableTestContext("POST", "/edit", `{"id":1,"name":"x"}`))

	if *hookCalls != 0 {
		t.Fatalf("AfterMutation ran %d times on failed Edit, want 0", *hookCalls)
	}
}

// Switch 部分更新：命中白名单时走 UpdateFieldScoped，不进入整行 Edit。
func TestTableHandlerPartialEdit(t *testing.T) {
	rows := &tableTestRows{row: tableTestRow{ID: 1, Name: "old", AdminID: 7}}
	h, hookCalls := newTableTestHandler(rows, map[string]bool{"name": true}, true)

	h.Edit(newTableTestContext("POST", "/edit", `{"id":1,"name":"p"}`))

	if !rows.updateCalled {
		t.Fatal("switch edit should call UpdateFieldScoped")
	}
	if rows.updatedField != "name" || rows.updatedValue != "p" {
		t.Fatalf("updated field = %q value = %v, want name/p", rows.updatedField, rows.updatedValue)
	}
	if len(rows.edited) != 0 {
		t.Fatalf("switch edit must not call rows.Edit, got %+v", rows.edited)
	}
	if *hookCalls != 0 {
		t.Fatalf("switch edit must not run AfterMutation, ran %d times", *hookCalls)
	}
}

// Add 成功写入后执行一次 hook；绑定失败时不写库也不执行 hook。
func TestTableHandlerAdd(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		rows := &tableTestRows{}
		h, hookCalls := newTableTestHandler(rows, nil, false)

		h.Add(newTableTestContext("POST", "/add", `{"name":"n"}`))

		if len(rows.added) != 1 || rows.added[0].Name != "n" {
			t.Fatalf("added rows = %+v, want one row name=n", rows.added)
		}
		if *hookCalls != 1 {
			t.Fatalf("AfterMutation ran %d times, want 1", *hookCalls)
		}
	})

	t.Run("绑定失败", func(t *testing.T) {
		rows := &tableTestRows{}
		h, hookCalls := newTableTestHandler(rows, nil, false)

		h.Add(newTableTestContext("POST", "/add", `{not json`))

		if len(rows.added) != 0 {
			t.Fatalf("invalid body must not reach rows.Add, got %+v", rows.added)
		}
		if *hookCalls != 0 {
			t.Fatalf("invalid body must not run AfterMutation, ran %d times", *hookCalls)
		}
	})
}

// Del 正常路径：解析全部 ids[]，写库后执行一次 hook。
func TestTableHandlerDel(t *testing.T) {
	rows := &tableTestRows{}
	h, hookCalls := newTableTestHandler(rows, nil, false)

	h.Del(newTableTestContext("DELETE", "/del?ids[]=1&ids[]=2", ""))

	if !rows.delCalled {
		t.Fatal("rows.Del was not called")
	}
	ids, ok := rows.deleted.([]int64)
	if !ok || len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("deleted ids = %v, want [1 2]", rows.deleted)
	}
	if *hookCalls != 1 {
		t.Fatalf("AfterMutation ran %d times, want 1", *hookCalls)
	}
}

// Del 参数缺失或非法时不得写库、不得执行 hook。
func TestTableHandlerDelRejectsInvalidIDs(t *testing.T) {
	for _, target := range []string{"/del", "/del?ids[]=x"} {
		rows := &tableTestRows{}
		h, hookCalls := newTableTestHandler(rows, nil, false)

		h.Del(newTableTestContext("DELETE", target, ""))

		if rows.delCalled {
			t.Fatalf("%q should not reach rows.Del", target)
		}
		if *hookCalls != 0 {
			t.Fatalf("%q must not run AfterMutation, ran %d times", target, *hookCalls)
		}
	}
}
