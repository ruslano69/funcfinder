package internal

type rustObjectPatch struct {
	start, end, edit int
	text             string
	items            []rustObjectItem
}

func planRustCodeEdits(edits []CodeEdit, src []byte) ([]byte, []CodeEditResult, error) {
	return planTokenCodeEdits(edits, src, "rust", rustCodeItems, rustObjectTokens)
}
