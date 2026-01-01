package utils

import (
	"testing"
)

func TestContainsJapanese(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Empty string", "", false},
		{"ASCII only", "Hello World", false},
		{"Hiragana", "こんにちは", true},
		{"Katakana", "コンニチハ", true},
		{"Kanji", "日本語", true},
		{"Mixed", "Hello こんにちは", true},
		{"Numbers", "12345", false},
		{"Symbols", "!@#$%", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsJapanese(tt.input)
			if result != tt.expected {
				t.Errorf("ContainsJapanese(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestJapaneseToRomaji(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"ASCII only", "Hello", "Hello"},
		{"Hiragana a-row", "あいうえお", "aiueo"},
		{"Hiragana ka-row", "かきくけこ", "kakikukeko"},
		{"Katakana a-row", "アイウエオ", "aiueo"},
		{"Katakana ka-row", "カキクケコ", "kakikukeko"},
		{"Mixed hiragana and katakana", "あいウエお", "aiueo"},
		{"Double consonant hiragana", "がっこう", "gakkou"},
		{"Double consonant katakana", "ロック", "rokku"},
		{"Combination sha", "しゃ", "sha"},
		{"Combination shu", "しゅ", "shu"},
		{"Combination sho", "しょ", "sho"},
		{"Katakana combination", "シャ", "sha"},
		{"Long vowel mark", "ラーメン", "ramen"},
		{"Mixed with ASCII", "Hello あい World", "Hello ai World"},
		{"Kanji kept as-is", "日本", "日本"},
		{"Mixed with kanji", "日本語あいう", "日本語aiu"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JapaneseToRomaji(tt.input)
			if result != tt.expected {
				t.Errorf("JapaneseToRomaji(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanToASCII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"ASCII only", "Hello World", "Hello World"},
		{"Numbers", "Test123", "Test123"},
		{"Hyphen", "Hello-World", "Hello-World"},
		{"Apostrophe", "It's", "It's"},
		{"Comma to space", "Hello,World", "Hello World"},
		{"Period to space", "Hello.World", "Hello World"},
		{"Japanese removed", "あいうえお", ""},
		{"Mixed keeps ASCII", "Hello あい World", "Hello World"},
		{"Multiple spaces cleaned", "Hello   World", "Hello World"},
		{"Special chars removed", "Hello@#$World", "HelloWorld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanToASCII(tt.input)
			if result != tt.expected {
				t.Errorf("CleanToASCII(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildJapaneseSearchQuery(t *testing.T) {
	tests := []struct {
		name       string
		trackName  string
		artistName string
		expected   string
	}{
		{"ASCII only", "Song", "Artist", "Artist Song"},
		{"Japanese track", "あいうえお", "Artist", "Artist aiueo"},
		{"Japanese artist", "Song", "あいうえお", "aiueo Song"},
		{"Both Japanese", "かきくけこ", "あいうえお", "aiueo kakikukeko"},
		{"With kanji (kanji removed)", "日本語", "アーティスト", "atisuto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildJapaneseSearchQuery(tt.trackName, tt.artistName)
			if result != tt.expected {
				t.Errorf("BuildJapaneseSearchQuery(%q, %q) = %q, want %q", tt.trackName, tt.artistName, result, tt.expected)
			}
		})
	}
}

func TestIsHiragana(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{'あ', true},
		{'ん', true},
		{'ア', false}, // katakana
		{'A', false},
		{'1', false},
	}

	for _, tt := range tests {
		result := isHiragana(tt.r)
		if result != tt.expected {
			t.Errorf("isHiragana(%q) = %v, want %v", tt.r, result, tt.expected)
		}
	}
}

func TestIsKatakana(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{'ア', true},
		{'ン', true},
		{'あ', false}, // hiragana
		{'A', false},
		{'1', false},
	}

	for _, tt := range tests {
		result := isKatakana(tt.r)
		if result != tt.expected {
			t.Errorf("isKatakana(%q) = %v, want %v", tt.r, result, tt.expected)
		}
	}
}

func TestIsKanji(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{'日', true},
		{'本', true},
		{'語', true},
		{'あ', false}, // hiragana
		{'ア', false}, // katakana
		{'A', false},
	}

	for _, tt := range tests {
		result := isKanji(tt.r)
		if result != tt.expected {
			t.Errorf("isKanji(%q) = %v, want %v", tt.r, result, tt.expected)
		}
	}
}
