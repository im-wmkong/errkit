package errkit_test

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/im-wmkong/errkit"
)

// 用独立 Registry, 避免不同测试间的 code/name 冲突。
func newReg() *errkit.Registry { return errkit.NewRegistry() }

// --- Identity / Instance --------------------------------------

func TestKindIdentityVsInstance(t *testing.T) {
	r := newReg()
	K := r.Define(1, "identity")

	e1 := K.New(errkit.Message("a"))
	e2 := K.New(errkit.Message("b"))
	if e1 == e2 {
		t.Fatal("instances must differ")
	}
	if !K.Is(e1) || !K.Is(e2) {
		t.Fatal("Kind.Is should match own instances")
	}
	if errkit.KindOf(e1) != K {
		t.Fatal("KindOf should return original Kind")
	}
}

// --- Define duplicates ----------------------------------------

func TestDefineDuplicateCode(t *testing.T) {
	r := newReg()
	r.Define(1, "a")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	r.Define(1, "b")
}

func TestDefineDuplicateName(t *testing.T) {
	r := newReg()
	r.Define(1, "a")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	r.Define(2, "a")
}

func TestDefineEmptyName(t *testing.T) {
	r := newReg()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	r.Define(1, "")
}

// --- Wrap nil -------------------------------------------------

func TestWrapNil(t *testing.T) {
	r := newReg()
	K := r.Define(1, "x")
	if got := K.Wrap(nil, errkit.Message("m")); got != nil {
		t.Fatalf("Wrap(nil) must return nil, got %v", got)
	}
}

// --- errors.Is / Kind.Is --------------------------------------

func TestErrorsIsCause(t *testing.T) {
	r := newReg()
	K := r.Define(1, "x")
	cause := stderrors.New("boom")
	e := K.Wrap(cause)
	if !stderrors.Is(e, cause) {
		t.Fatal("errors.Is should find cause")
	}
	if !K.Is(e) {
		t.Fatal("Kind.Is failed")
	}
}

func TestKindIsThroughChain(t *testing.T) {
	r := newReg()
	A := r.Define(1, "a")
	B := r.Define(2, "b")
	e := B.Wrap(A.New())
	if !A.Is(e) || !B.Is(e) {
		t.Fatal("both Kind.Is should match through chain")
	}
}

// --- KindOf / MessageOf / AttrsOf / AllAttrs ------------------

func TestKindOf(t *testing.T) {
	r := newReg()
	K := r.Define(1, "x")
	e := K.New()
	if got := errkit.KindOf(e); got != K {
		t.Fatal("KindOf wrong")
	}
	if got := errkit.KindOf(stderrors.New("plain")); got != nil {
		t.Fatal("KindOf on plain error should be nil")
	}
}

func TestCodeOfAndNameOf(t *testing.T) {
	r := newReg()
	K := r.Define(42, "named")
	e := K.New()

	if c, ok := errkit.CodeOf(e); !ok || c != 42 {
		t.Fatalf("CodeOf wrong: %v %v", c, ok)
	}
	if n, ok := errkit.NameOf(e); !ok || n != "named" {
		t.Fatalf("NameOf wrong: %v %v", n, ok)
	}

	if _, ok := errkit.CodeOf(stderrors.New("plain")); ok {
		t.Fatal("CodeOf on plain error should be false")
	}
	if _, ok := errkit.NameOf(nil); ok {
		t.Fatal("NameOf(nil) should be false")
	}
}

func TestAttrsOfReturnsCopy(t *testing.T) {
	r := newReg()
	K := r.Define(1, "x")
	e := K.New(errkit.With("uid", 1))

	got := errkit.AttrsOf(e)
	got[0].Val = 999 // 改外部副本

	got2 := errkit.AttrsOf(e)
	if got2[0].Val != 1 {
		t.Fatalf("AttrsOf must return a copy, got %v", got2[0].Val)
	}
}

func TestMessageOfFallback(t *testing.T) {
	if got := errkit.MessageOf(stderrors.New("raw")); got != "raw" {
		t.Fatalf("fallback to err.Error(), got %q", got)
	}
	if got := errkit.MessageOf(nil); got != "" {
		t.Fatal("nil should be empty")
	}
}

func TestAttrsOrderAndOverride(t *testing.T) {
	r := newReg()
	K := r.Define(1, "x")
	e := K.New(
		errkit.With("uid", 1),
		errkit.With("trace", "x"),
		errkit.With("uid", 2),
	)
	attrs := errkit.AttrsOf(e)
	if len(attrs) != 2 {
		t.Fatalf("len wrong: %v", attrs)
	}
	if attrs[0].Key != "uid" || attrs[0].Val != 2 {
		t.Fatalf("first wrong: %v", attrs[0])
	}
	if attrs[1].Key != "trace" {
		t.Fatalf("second wrong: %v", attrs[1])
	}
}

func TestAllAttrsFlatten(t *testing.T) {
	r := newReg()
	A := r.Define(1, "a")
	B := r.Define(2, "b")
	e := B.Wrap(A.New(errkit.With("inner", 1), errkit.With("shared", "from_a")),
		errkit.With("outer", 2),
		errkit.With("shared", "from_b"))
	all := errkit.AllAttrs(e)

	got := map[string]any{}
	for _, a := range all {
		got[a.Key] = a.Val
	}
	if got["inner"] != 1 || got["outer"] != 2 {
		t.Fatalf("flatten wrong: %+v", got)
	}
	if got["shared"] != "from_b" {
		t.Fatalf("outer should win: %v", got["shared"])
	}
}

// --- DefaultMessage --------------------------------------------

func TestDefaultMessage(t *testing.T) {
	r := newReg()
	K := r.Define(1, "x", errkit.DefaultMessage("默认"))
	if errkit.MessageOf(K.New()) != "默认" {
		t.Fatal("default message")
	}
	if errkit.MessageOf(K.New(errkit.Message("覆盖"))) != "覆盖" {
		t.Fatal("override")
	}
}

// --- Stack (进程级开关) ----------------------------------------

func TestStackDefaultOff(t *testing.T) {
	r := newReg()
	K := r.Define(1, "x")
	e := K.New()
	st := e.(errkit.Tracer).StackTrace()
	if len(st) != 0 {
		t.Fatal("default should not capture stack")
	}
}

func TestStackOn(t *testing.T) {
	errkit.SetCaptureStack(true)
	t.Cleanup(func() { errkit.SetCaptureStack(false) })

	r := newReg()
	K := r.Define(1, "x")
	e := K.New()
	st := e.(errkit.Tracer).StackTrace()
	if len(st) == 0 {
		t.Fatal("expected frames")
	}
	if !strings.Contains(st[0].Function, "TestStackOn") &&
		!strings.Contains(st[1].Function, "TestStackOn") {
		t.Fatalf("expected TestStackOn near top, got %v", st)
	}
}

// --- Registry helpers -----------------------------------------

func TestLookup(t *testing.T) {
	r := newReg()
	K := r.Define(42, "lookup")
	if r.LookupCode(42) != K {
		t.Fatal("LookupCode")
	}
	if r.LookupName("lookup") != K {
		t.Fatal("LookupName")
	}
	if r.LookupCode(999) != nil {
		t.Fatal("missing code should be nil")
	}
	if len(r.Kinds()) != 1 {
		t.Fatal("Kinds count wrong")
	}
}

// --- Error 文本 -----------------------------------------------

func TestErrorString(t *testing.T) {
	r := newReg()
	K := r.Define(7, "boom")
	e := K.Wrap(stderrors.New("root"), errkit.Message("ctx"))
	got := e.Error()
	for _, w := range []string{"boom", "(7)", "ctx", "root"} {
		if !strings.Contains(got, w) {
			t.Fatalf("missing %q in %q", w, got)
		}
	}
}

// --- Format (%v / %+v / %q) -----------------------------------

func TestFormatV(t *testing.T) {
	r := newReg()
	K := r.Define(1, "fmt_v")
	e := K.New(errkit.Message("m"))
	if got := fmt.Sprintf("%v", e); got != e.Error() {
		t.Fatalf("%%v should equal Error(), got %q", got)
	}
	if got := fmt.Sprintf("%s", e); got != e.Error() {
		t.Fatalf("%%s should equal Error(), got %q", got)
	}
}

func TestFormatQ(t *testing.T) {
	r := newReg()
	K := r.Define(1, "fmt_q")
	e := K.New(errkit.Message("m"))
	got := fmt.Sprintf("%q", e)
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("%%q should be quoted, got %s", got)
	}
}

func TestFormatPlusVWithStack(t *testing.T) {
	errkit.SetCaptureStack(true)
	t.Cleanup(func() { errkit.SetCaptureStack(false) })

	r := newReg()
	K := r.Define(1, "fmt_plus")
	e := K.New(errkit.Message("m"))

	got := fmt.Sprintf("%+v", e)
	if !strings.Contains(got, "fmt_plus") {
		t.Fatalf("missing kind name in %%+v: %q", got)
	}
	// 必须出现栈帧 (file:line 形式)
	if !strings.Contains(got, ".go:") {
		t.Fatalf("expected stack frames in %%+v, got %q", got)
	}
}

func TestFormatPlusVWithoutStack(t *testing.T) {
	r := newReg()
	K := r.Define(1, "fmt_plus_nostack")
	e := K.New(errkit.Message("m"))
	got := fmt.Sprintf("%+v", e)
	// 没栈时 %+v 等于 Error()
	if got != e.Error() {
		t.Fatalf("without stack, %%+v should equal Error(); got %q", got)
	}
}

// --- JSON ----------------------------------------------------

func TestMarshalJSON(t *testing.T) {
	r := newReg()
	K := r.Define(10001, "json_basic", errkit.DefaultMessage("默认"))
	e := K.Wrap(stderrors.New("root"),
		errkit.With("uid", 42),
		errkit.With("name", "alice"),
	)

	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// 验证字段
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, raw)
	}
	if int(m["code"].(float64)) != 10001 {
		t.Fatalf("code wrong: %v", m["code"])
	}
	if m["name"] != "json_basic" {
		t.Fatalf("name wrong: %v", m["name"])
	}
	if m["message"] != "默认" {
		t.Fatalf("message wrong: %v", m["message"])
	}
	if m["cause"] != "root" {
		t.Fatalf("cause wrong: %v", m["cause"])
	}
	attrs := m["attrs"].(map[string]any)
	if int(attrs["uid"].(float64)) != 42 || attrs["name"] != "alice" {
		t.Fatalf("attrs wrong: %v", attrs)
	}

	// attrs 顺序: uid 先于 name
	s := string(raw)
	if strings.Index(s, `"uid"`) > strings.Index(s, `"name":"alice"`) {
		t.Fatalf("attrs order broken: %s", s)
	}
}

func TestMarshalJSONNoCauseNoMessageNoAttrs(t *testing.T) {
	r := newReg()
	K := r.Define(1, "json_minimal")
	raw, err := json.Marshal(K.New())
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	want := `{"code":1,"name":"json_minimal"}`
	if got != want {
		t.Fatalf("minimal JSON wrong:\nwant %s\n got %s", want, got)
	}
}

func TestMarshalJSONNestedErrkit(t *testing.T) {
	r := newReg()
	A := r.Define(1, "json_inner")
	B := r.Define(2, "json_outer")
	e := B.Wrap(A.New(errkit.With("a", 1)), errkit.With("b", 2))

	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `"name":"json_outer"`) {
		t.Fatalf("missing outer: %s", s)
	}
	if !strings.Contains(s, `"name":"json_inner"`) {
		t.Fatalf("inner cause should be expanded: %s", s)
	}
}

func TestMarshalJSONUnserializableValue(t *testing.T) {
	r := newReg()
	K := r.Define(1, "json_bad_attr")

	// chan 不可被 json.Marshal, 应当降级为字符串而不是整条失败
	bad := make(chan int)
	e := K.New(errkit.With("ch", bad))

	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("should not fail, got %v", err)
	}
	if !strings.Contains(string(raw), `"ch":`) {
		t.Fatalf("missing ch attr: %s", raw)
	}
}

// 自定义 json.Marshaler, 用来覆盖 (*kerr).MarshalJSON 中
// "cause 实现 json.Marshaler 但不是 *kerr" 的分支。
type jsonCause struct{ msg string }

func (c *jsonCause) Error() string                { return c.msg }
func (c *jsonCause) MarshalJSON() ([]byte, error) { return []byte(`{"x":` + jsonQuote(c.msg) + `}`), nil }
func jsonQuote(s string) string                   { return `"` + s + `"` }

func TestMarshalJSONCauseImplementsMarshaler(t *testing.T) {
	r := newReg()
	K := r.Define(1, "json_cause_marshaler")
	e := K.Wrap(&jsonCause{msg: "ext"})

	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"cause":{"x":"ext"}`) {
		t.Fatalf("cause should be expanded JSON, got %s", raw)
	}
}

// --- Kind 三个 trivial accessor + Messagef ---------------------

func TestKindAccessors(t *testing.T) {
	r := newReg()
	K := r.Define(7, "accessors", errkit.DefaultMessage("默认"))
	if K.Code() != 7 {
		t.Fatal("Code")
	}
	if K.Name() != "accessors" {
		t.Fatal("Name")
	}
	if K.DefaultMessage() != "默认" {
		t.Fatal("DefaultMessage")
	}
}

func TestMessagef(t *testing.T) {
	r := newReg()
	K := r.Define(1, "msgf")
	e := K.New(errkit.Messagef("uid=%d age=%d", 42, 18))
	if errkit.MessageOf(e) != "uid=42 age=18" {
		t.Fatalf("Messagef wrong: %q", errkit.MessageOf(e))
	}
}

// --- *kerr 的 trivial accessor 通过类型断言访问 ---------------

// 用 errors.As 提取出来后调用 Kind/Message/Attrs 也应工作。
type kerrView interface {
	Kind() *errkit.Kind
	Message() string
	Attrs() []errkit.Attr
}

func TestKerrViewAccessors(t *testing.T) {
	r := newReg()
	K := r.Define(1, "kerr_view")
	e := K.New(errkit.Message("m"), errkit.With("uid", 1))

	v, ok := e.(kerrView)
	if !ok {
		t.Fatal("kerr should expose Kind/Message/Attrs")
	}
	if v.Kind() != K {
		t.Fatal("Kind()")
	}
	if v.Message() != "m" {
		t.Fatal("Message()")
	}
	if len(v.Attrs()) != 1 || v.Attrs()[0].Key != "uid" {
		t.Fatalf("Attrs(): %v", v.Attrs())
	}
}

// --- AttrsOf 空 attr / 非 errkit 错误 -------------------------

func TestAttrsOfEmptyAndPlain(t *testing.T) {
	r := newReg()
	K := r.Define(1, "attrs_empty")
	if got := errkit.AttrsOf(K.New()); got != nil {
		t.Fatalf("no attrs should return nil, got %v", got)
	}
	if got := errkit.AttrsOf(stderrors.New("plain")); got != nil {
		t.Fatalf("plain error should return nil, got %v", got)
	}
}

// --- AllAttrs 穿过非 errkit 节点 ------------------------------

func TestAllAttrsThroughNonErrkitWrap(t *testing.T) {
	r := newReg()
	K := r.Define(1, "allattrs_chain")
	inner := K.New(errkit.With("a", 1))
	// 用标准 fmt.Errorf 包一层非 errkit, 验证 AllAttrs 能穿过
	mid := fmt.Errorf("mid: %w", inner)
	if got := errkit.AllAttrs(mid); len(got) != 1 || got[0].Key != "a" {
		t.Fatalf("should walk through non-errkit nodes, got %v", got)
	}
	// 错误链终止时也要正常退出
	if got := errkit.AllAttrs(stderrors.New("plain")); got != nil {
		t.Fatalf("plain should be nil, got %v", got)
	}
	if got := errkit.AllAttrs(nil); got != nil {
		t.Fatalf("nil should be nil, got %v", got)
	}
}

// --- Kind.Is 在非 errkit 链上正确返回 false -------------------

func TestKindIsOnNonErrkit(t *testing.T) {
	r := newReg()
	K := r.Define(1, "is_negative")
	if K.Is(stderrors.New("plain")) {
		t.Fatal("plain error should not match")
	}
	if K.Is(nil) {
		t.Fatal("nil should not match")
	}
	// 一条链上不含 K 的实例
	other := r.Define(2, "is_other")
	e := other.New()
	if K.Is(e) {
		t.Fatal("different Kind should not match")
	}
}

// --- 包级默认 Registry 也走一遍 ------------------------------

func TestPackageLevelRegistry(t *testing.T) {
	// 用一个不太可能冲突的 code 段
	K := errkit.Define(900001, "pkg_level_define")
	if got := errkit.LookupCode(900001); got != K {
		t.Fatal("LookupCode")
	}
	if got := errkit.LookupName("pkg_level_define"); got != K {
		t.Fatal("LookupName")
	}
	all := errkit.Kinds()
	found := false
	for _, k := range all {
		if k == K {
			found = true
		}
	}
	if !found {
		t.Fatal("Kinds() should include defined kind")
	}
}

// --- hasStack: Wrap 一个已抓栈错误时不重复抓 ------------------

// fakeTracer 让 hasStack 找到 Tracer; cause 链里出现它时, 新 Wrap 不应再抓。
type fakeTracer struct{ msg string }

func (f *fakeTracer) Error() string             { return f.msg }
func (f *fakeTracer) StackTrace() []errkit.Frame { return []errkit.Frame{{Function: "fake", File: "f.go", Line: 1}} }

func TestHasStackPreventsRecapture(t *testing.T) {
	errkit.SetCaptureStack(true)
	t.Cleanup(func() { errkit.SetCaptureStack(false) })

	r := newReg()
	K := r.Define(1, "has_stack_skip")
	e := K.Wrap(&fakeTracer{msg: "boom"}).(errkit.Tracer)

	// 新 *kerr 因为 cause 已实现 Tracer, 自身不抓栈
	if len(e.StackTrace()) != 0 {
		t.Fatalf("should not capture again, got %d frames", len(e.StackTrace()))
	}
}

// --- Frame.String 格式 ---------------------------------------

func TestFrameString(t *testing.T) {
	f := errkit.Frame{Function: "pkg.Func", File: "/x/y.go", Line: 42}
	want := "pkg.Func\n\t/x/y.go:42"
	if f.String() != want {
		t.Fatalf("want %q got %q", want, f.String())
	}
}
