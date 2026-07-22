package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
	"ttgo/internal/config"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const resetPasswordUsage = `Usage: <server binary> reset-password <email>

Break-glass password recovery for when nobody who could reset a password is
able to sign in (typically a sole administrator). Run it on the server host:
it opens the database directly, honouring DB_PATH and .env exactly like the
server, so no login is required — shell access to the host is the authorization.

It resets the account's password to a generated temporary one, re-enables the
account if it was deactivated or soft-deleted, and signs out all of its
sessions. The server can keep running; the new password works immediately.
`

// tempPasswordAlphabet deliberately omits look-alikes (0/O, 1/l/I) because the
// operator retypes the printed password by hand exactly once.
const tempPasswordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateTempPassword(n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(tempPasswordAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = tempPasswordAlphabet[idx.Int64()]
	}
	return string(b), nil
}

// runResetPassword implements `reset-password <email>` and returns the process
// exit code. Output goes to the given writers so tests can capture it.
func runResetPassword(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, resetPasswordUsage)
		return 0
	}
	if len(args) != 1 {
		fmt.Fprint(stderr, resetPasswordUsage)
		return 2
	}
	email := args[0]

	cfg := config.Load()
	if _, err := os.Stat(cfg.DBPath); err != nil {
		abs, _ := filepath.Abs(cfg.DBPath)
		fmt.Fprintf(stderr, "Database not found at %s.\nRun this from the server's working directory, or set DB_PATH to the database file.\n", abs)
		return 1
	}

	s, err := store.New(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to open database: %v\n", err)
		return 1
	}
	defer func() { _ = s.Close() }()

	user, err := s.FindUserByEmail(email)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to look up user: %v\n", err)
		return 1
	}
	if user == nil {
		fmt.Fprintf(stderr, "No user with email %q.\n", email)
		if admins := adminEmails(s); len(admins) > 0 {
			fmt.Fprintf(stderr, "Administrators on this instance: %s\n", strings.Join(admins, ", "))
		}
		return 1
	}

	tempPassword, err := generateTempPassword(20)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to generate a password: %v\n", err)
		return 1
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), 12)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to hash the password: %v\n", err)
		return 1
	}

	if _, err := s.RecoverUserAccess(user.ID, string(hash)); err != nil {
		fmt.Fprintf(stderr, "Failed to reset the password: %v\n", err)
		return 1
	}

	// Same action name as an admin-initiated reset; the diff marks the actorless
	// break-glass path so the audit trail distinguishes the two.
	_ = s.CreateAuditLog(&models.AuditLog{
		ID:        uuid.New().String(),
		UserID:    "",
		Action:    "user.password_reset",
		Diff:      "target=" + user.ID + " method=break_glass_cli",
		Timestamp: time.Now(),
	})

	fmt.Fprintf(stdout, "Password reset for %s\n\n", user.Email)
	fmt.Fprintf(stdout, "  Temporary password: %s\n\n", tempPassword)
	if !user.Active {
		fmt.Fprintln(stdout, "  - the account was deactivated and has been re-enabled")
	}
	if user.Deleted {
		fmt.Fprintln(stdout, "  - the account was soft-deleted and has been restored")
	}
	fmt.Fprintln(stdout, "  - all existing sessions for this account were signed out")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Sign in with the temporary password, then change it in Settings → Account.")
	return 0
}

// adminEmails lists non-deleted admin accounts for the "no such user" hint —
// anyone running this already has full read access to the database file.
func adminEmails(s *store.Store) []string {
	users, err := s.ListUsers(false)
	if err != nil {
		return nil
	}
	var out []string
	for _, u := range users {
		if u.Role == "admin" {
			out = append(out, u.Email)
		}
	}
	return out
}
