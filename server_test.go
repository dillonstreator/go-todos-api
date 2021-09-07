package main

import (
	"os"
	"testing"
)

func Test_onlySomeEnvsSet(t *testing.T) {
	t.Run("false when no envs passed in", func(t *testing.T) {
		actual := onlySomeEnvsSet()
		expected := false
		if expected != actual {
			t.Errorf("onlySomeEnvsSet() = %v, expected %v", actual, expected)
		}
	})
	t.Run("true when only some of the envs are set", func(t *testing.T) {
		os.Setenv("valueOne", "one")
		actual := onlySomeEnvsSet("valueOne", "valueTwo")
		expected := true
		if expected != actual {
			t.Errorf("onlySomeEnvsSet() = %v, expected %v", actual, expected)
		}
		os.Unsetenv("valueOne")
	})
	t.Run("false when all envs are set", func(t *testing.T) {
		os.Setenv("valueOne", "one")
		os.Setenv("valueTwo", "two")
		actual := onlySomeEnvsSet("valueOne", "valueTwo")
		expected := false
		if expected != actual {
			t.Errorf("onlySomeEnvsSet() = %v, expected %v", actual, expected)
		}
		os.Unsetenv("valueOne")
		os.Unsetenv("valueTwo")
	})
	t.Run("false when none of the envs are set", func(t *testing.T) {
		actual := onlySomeEnvsSet("valueOne", "valueTwo")
		expected := false
		if expected != actual {
			t.Errorf("onlySomeEnvsSet() = %v, expected %v", actual, expected)
		}
	})
}

func Test_noEnvsSet(t *testing.T) {
	t.Run("true when none of the envs are set", func(t *testing.T) {
		actual := noEnvsSet("valueOne", "valueTwo")
		expected := true
		if expected != actual {
			t.Errorf("noEnvsSet() = %v, expected %v", actual, expected)
		}
	})
	t.Run("false when some of the envs set", func(t *testing.T) {
		os.Setenv("valueOne", "one")
		actual := noEnvsSet("valueOne", "valueTwo")
		expected := false
		if expected != actual {
			t.Errorf("noEnvsSet() = %v, expected %v", actual, expected)
		}
		os.Unsetenv("valueOne")
	})
	t.Run("false when all of the envs set", func(t *testing.T) {
		os.Setenv("valueOne", "one")
		os.Setenv("valueTwo", "two")
		actual := noEnvsSet("valueOne", "valueTwo")
		expected := false
		if expected != actual {
			t.Errorf("noEnvsSet() = %v, expected %v", actual, expected)
		}
		os.Unsetenv("valueOne")
		os.Unsetenv("valueTwo")
	})
}
