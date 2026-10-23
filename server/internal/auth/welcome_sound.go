package auth

import (
	"errors"
	"time"

	"bedrud/internal/models"
	"bedrud/internal/storage"
)

const (
	// welcomeSoundChangesPerDay caps uploads per UTC calendar day; clearing never counts.
	welcomeSoundChangesPerDay = 2
	welcomeSoundDayLayout     = "2006-01-02"
)

var ErrWelcomeSoundDailyLimit = errors.New("you can change your welcome sound twice a day; try again tomorrow")

func (s *AuthService) findUser(userID string) (*models.User, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// SetWelcomeSound validates and stores a new welcome sound, counting it against today's allowance.
// A clip that fails validation leaves the allowance untouched.
func (s *AuthService) SetWelcomeSound(userID string, data []byte, limits storage.WelcomeSoundLimits, now time.Time) (*models.User, error) {
	user, err := s.findUser(userID)
	if err != nil {
		return nil, err
	}

	today := now.UTC().Format(welcomeSoundDayLayout)
	changesToday := 0
	if user.WelcomeSoundChangeDay == today {
		changesToday = user.WelcomeSoundChangeCount
	}
	if changesToday >= welcomeSoundChangesPerDay {
		return nil, ErrWelcomeSoundDailyLimit
	}

	url, err := storage.SaveUserWelcomeSound(userID, data, limits)
	if err != nil {
		return nil, err
	}
	user.WelcomeSoundURL = url
	user.WelcomeSoundChangeDay = today
	user.WelcomeSoundChangeCount = changesToday + 1
	if err := s.userRepo.UpdateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

// ClearWelcomeSound removes the user's welcome sound file and URL.
func (s *AuthService) ClearWelcomeSound(userID string) (*models.User, error) {
	user, err := s.findUser(userID)
	if err != nil {
		return nil, err
	}
	if err := storage.DeleteUserWelcomeSoundFile(userID); err != nil {
		return nil, err
	}
	user.WelcomeSoundURL = ""
	if err := s.userRepo.UpdateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}
