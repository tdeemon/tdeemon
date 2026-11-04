package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/tdeemon/tdeemon/internal/auth"
	"github.com/tdeemon/tdeemon/internal/config"
	"github.com/tdeemon/tdeemon/internal/demo"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
)

type demoFlags struct {
	createUserFlags
	days int
	seed uint64
}

func parseDemoFlags(args []string) (demoFlags, error) {
	var f demoFlags
	set := flag.NewFlagSet("user demo", flag.ContinueOnError)
	set.SetOutput(io.Discard) // exitCode prints the error and the usage.
	set.StringVar(&f.email, "email", "", "")
	set.StringVar(&f.name, "name", "", "")
	set.StringVar(&f.timeZone, "timezone", "", "")
	set.StringVar(&f.locale, "locale", user.DefaultLocale, "")
	set.BoolVar(&f.passwordStdin, "password-stdin", false, "")
	set.IntVar(&f.days, "days", demo.DefaultDays, "")
	set.Uint64Var(&f.seed, "seed", demo.DefaultSeed, "")
	if err := set.Parse(args); errors.Is(err, flag.ErrHelp) {
		return f, err
	} else if err != nil {
		return f, fmt.Errorf("%w: user demo: %w", errUsage, err)
	}
	if set.NArg() > 0 {
		return f, fmt.Errorf("%w: user demo: unexpected argument %q", errUsage, set.Arg(0))
	}
	if f.email == "" {
		return f, fmt.Errorf("%w: user demo: --email is required", errUsage)
	}
	if f.days < demo.MinDays || f.days > demo.MaxDays {
		return f, fmt.Errorf("%w: user demo: --days %d: %w", errUsage, f.days, demo.ErrDays)
	}
	return f, nil
}

// demoUser adds a user with --days of made-up data (package demo), for
// trying tdeemon or for screenshots. Like createUser, it never migrates;
// a user whose data fail to be written is deleted again.
func demoUser(ctx context.Context, args []string, lookup config.LookupFunc, in io.Reader, out io.Writer) error {
	f, err := parseDemoFlags(args)
	if errors.Is(err, flag.ErrHelp) {
		_, err = io.WriteString(out, usage)
		return err
	} else if err != nil {
		return err
	}
	cfg, err := config.Load(lookup)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	now := time.Now()
	// The account is as old as its data.
	u := newUser(f.createUserFlags, cfg.Location, now.AddDate(0, 0, -f.days-1))
	if err := checkNewUser(u, f.name != ""); err != nil {
		return err
	}
	password, generated, err := choosePassword(f.passwordStdin, in)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(ctx, password)
	if err != nil {
		return err
	}

	var res demo.Result
	err = withDatabase(ctx, cfg.DataDir, func(db *store.DB) error {
		if err := db.CreateUser(ctx, u, hash); errors.Is(err, store.ErrEmailTaken) {
			return fmt.Errorf("%s: %w", u.Email, err)
		} else if err != nil {
			return err
		}
		res, err = demo.Seed(ctx, db, u, demo.Options{Days: f.days, Seed: f.seed, Now: now, Instance: cfg.Limits})
		if err != nil {
			_, derr := db.DeleteUser(ctx, u.ID)
			return errors.Join(fmt.Errorf("demo data: %w", err), derr)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Created %s %s (id %s) with demo data from %s: %d meals, %d weigh-ins.\n",
		u.Role, u.Email, u.ID, res.Start, res.Meals, res.WeighIns); err != nil {
		return err
	}
	return printPassword(out, password, generated)
}
