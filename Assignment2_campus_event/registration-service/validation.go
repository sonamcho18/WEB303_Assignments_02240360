package main

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	pb "campus-events/gen/registrationpb"
)

// Student numbers are exactly 8 digits, e.g. 02240365.
var studentIDPattern = regexp.MustCompile(`^[0-9]{8}$`)

func validateCreate(req *pb.CreateRegistrationRequest) error {
	req.StudentName = strings.TrimSpace(req.StudentName)
	req.StudentId = strings.TrimSpace(req.StudentId)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var problems []string
	if req.EventId <= 0 {
		problems = append(problems, "event_id is required and must be a positive number")
	}
	problems = append(problems, checkName(req.StudentName)...)
	if req.StudentId == "" {
		problems = append(problems, "student_id is required")
	} else if !studentIDPattern.MatchString(req.StudentId) {
		problems = append(problems, "student_id must be exactly 8 digits, e.g. 02240365")
	}
	problems = append(problems, checkEmail(req.Email)...)
	return join(problems)
}

func validateUpdate(req *pb.UpdateRegistrationRequest) error {
	req.StudentName = strings.TrimSpace(req.StudentName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var problems []string
	if req.Id <= 0 {
		problems = append(problems, "id must be a positive number")
	}
	problems = append(problems, checkName(req.StudentName)...)
	problems = append(problems, checkEmail(req.Email)...)
	return join(problems)
}

func checkName(name string) []string {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return []string{"student_name is required"}
	case n < 2 || n > 100:
		return []string{"student_name must be between 2 and 100 characters"}
	}
	return nil
}

func checkEmail(email string) []string {
	if email == "" {
		return []string{"email is required"}
	}
	// ParseAddress also accepts "Name <a@b.c>", so insist on a bare address
	// that contains a domain with a dot.
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
		return []string{"email must be a valid email address, e.g. name@rub.edu.bt"}
	}
	return nil
}

func join(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "; "))
}
