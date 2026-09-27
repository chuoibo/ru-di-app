package tools

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

// BoCongCu builds the turn's ADK tools: one functiontool per tool the bot
// may call this turn (DuocPhep), in registry order, each declared with the
// contract's argument schema and the long description. A tool the bot may
// not call is not declared at all, so the model never sees it.
func (bc *BoiCanh) BoCongCu() ([]tool.Tool, error) {
	bc.mu.Lock()
	bc.khoiTao()
	bc.mu.Unlock()
	var out []tool.Tool
	for _, t := range bc.DuocPhep() {
		tt, err := congCus[t].moi(bc)
		if err != nil {
			return nil, err
		}
		out = append(out, tt)
	}
	return out, nil
}

// TruocTool is the llmagent BeforeToolCallback: a non-nil map is the
// tool's answer and the tool does not run (a refusal, or the first result
// of an identical call).
func (bc *BoiCanh) TruocTool(_ agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	return bc.truoc(t.Name(), args), nil
}

// SauTool is the llmagent AfterToolCallback: it records a finished call's
// evidence in the ledger and renders it for the model. A refusal or an
// error answer passes through unchanged.
func (bc *BoiCanh) SauTool(tc agent.Context, t tool.Tool, args, _ map[string]any, err error) (map[string]any, error) {
	if err != nil {
		return bc.loiChay(t.Name(), err), nil
	}
	if r := bc.sau(tc.FunctionCallID(), args); r != nil {
		return r, nil
	}
	return nil, nil
}

// LoiTool is the llmagent OnToolErrorCallback. ADK calls it for a name the
// toolset does not have (an invented or unpermitted tool) and for a run
// that failed; the model gets a closed code, never the error's text or the
// list of tools ADK would otherwise print.
func (bc *BoiCanh) LoiTool(_ agent.Context, t tool.Tool, _ map[string]any, err error) (map[string]any, error) {
	ten := t.Name()
	ok := false
	if x, e := Tens.Parse(ten); e == nil {
		for _, d := range bc.DuocPhep() {
			ok = ok || d == x
		}
	}
	if !ok {
		return bc.tuChoi(ten, &loiTS{loi: KhongDuocPhep, sua: true}), nil
	}
	return bc.loiChay(ten, err), nil
}
