package main

import (
	"os"
	"testing"
)

func TestLoadEnvToleratesBOMAndQuotes(t *testing.T) {
	t.Chdir(t.TempDir())
	env := "\ufeffTESSERA_TEST_A=plain\r\n" +
		"# comment\n" +
		"TESSERA_TEST_B=\"double quoted\"\n" +
		"TESSERA_TEST_C='single quoted'\n" +
		"TESSERA_TEST_KEEP=from-file\n"
	if err := os.WriteFile(".env", []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TESSERA_TEST_KEEP", "from-process")
	for _, k := range []string{"TESSERA_TEST_A", "TESSERA_TEST_B", "TESSERA_TEST_C"} {
		t.Cleanup(func() { os.Unsetenv(k) })
	}

	loadEnv()

	want := map[string]string{
		"TESSERA_TEST_A":    "plain",
		"TESSERA_TEST_B":    "double quoted",
		"TESSERA_TEST_C":    "single quoted",
		"TESSERA_TEST_KEEP": "from-process", // existing env is not overridden
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}
