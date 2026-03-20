package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	storm "github.com/asdine/storm/v3"
	"github.com/bambuvnn/filebrowser/backend/adapters/fs/fileutils"
	"github.com/bambuvnn/filebrowser/backend/common/settings"
	"github.com/bambuvnn/filebrowser/backend/common/utils"
	"github.com/bambuvnn/filebrowser/backend/database/storage/bolt"
	"github.com/bambuvnn/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

var userStore *users.Storage

func InitializeDb(path string) (*bolt.BoltStore, bool, error) {
	logger.Debugf("Initializing database: %v", path)
	exists, err := dbExists(path)
	if err != nil {
		panic(err)
	}
	db, err := storm.Open(path)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			logger.Fatal("the database is locked, please close all other instances of filebrowser before starting.")
		}
		logger.Fatalf("could not open database: %v", err)
	}
	store, err := bolt.NewStorage(db)
	if err != nil {
		return nil, exists, err
	}
	// Load access rules from DB on startup
	// ignoring errors because
	_ = store.Access.LoadFromDB()
	// Load share cache from DB so shareByHash/shareByPath are populated at startup
	store.Share.LoadShareCacheFromDB()
	userStore = store.Users
	err = bolt.Save(db, "version", 2)
	if err != nil {
		return nil, exists, err
	}
	if !exists {
		if settings.Env.IsPlaywright || settings.Env.IsDevMode {
			settings.Env.IsFirstLoad = false
		} else {
			settings.Env.IsFirstLoad = true
		}
		quickSetup(store)
	} else {
		// Load Auth.Key from database when DB already exists and no env var override is set.
		// This ensures JWT tokens signed before a restart can still be verified.
		if len(settings.Config.Auth.Key) == 0 {
			savedSettings, err := store.Settings.Get()
			if err == nil && savedSettings != nil && len(savedSettings.Auth.Key) > 0 {
				settings.Config.Auth.Key = savedSettings.Auth.Key
				logger.Debug("Loaded JWT signing key from database")
			} else {
				// No key in DB either — generate and persist one so future restarts are consistent
				settings.Config.Auth.Key = utils.GenerateKey()
				if saveErr := store.Settings.Save(&settings.Config); saveErr != nil {
					logger.Errorf("Failed to save generated JWT signing key to database: %v", saveErr)
				} else {
					logger.Info("Generated and saved new JWT signing key to database")
				}
			}
		}
	}

	return store, exists, err
}

func dbExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == nil {
		return stat.Size() != 0, nil
	}

	if os.IsNotExist(err) {
		d := filepath.Dir(path)
		_, err = os.Stat(d)
		if os.IsNotExist(err) {
			if err = os.MkdirAll(d, fileutils.PermDir); err != nil {
				return false, err
			}
			return false, nil
		}
	}

	return false, err
}

func quickSetup(store *bolt.BoltStore) {
	settings.Config.Auth.Key = utils.GenerateKey()
	err := store.Settings.Save(&settings.Config)
	utils.CheckErr("store.Settings.Save", err)
	err = store.Settings.SaveServer(&settings.Config.Server)
	utils.CheckErr("store.Settings.SaveServer", err)
	passwordAuth := settings.Config.Auth.Methods.PasswordAuth.Enabled
	noAuth := settings.Config.Auth.Methods.NoAuth
	if passwordAuth || noAuth {
		user := &users.User{}
		settings.ApplyUserDefaults(user)
		user.Username = settings.Config.Auth.AdminUsername
		if settings.Config.Auth.AdminPassword == "" {
			settings.Config.Auth.AdminPassword = "admin"
		}
		user.Password = settings.Config.Auth.AdminPassword
		user.Permissions.Admin = true
		user.Scopes = []users.SourceScope{}
		for _, val := range settings.Config.Server.Sources {
			user.Scopes = append(user.Scopes, users.SourceScope{
				Name:  val.Path, // backend name is path
				Scope: "",
			})
		}
		user.LockPassword = false
		user.Permissions = settings.AdminPerms()
		user.ShowFirstLogin = settings.Env.IsFirstLoad && user.Permissions.Admin
		logger.Debugf("Creating user as admin: %v %v", user.Username, user.Password)
		err = store.Users.Save(user, true, true)
		utils.CheckErr("store.Users.Save", err)
	}
}

// create new user
func CreateUser(userInfo users.User, permissions users.Permissions) error {
	newUser := &userInfo
	newUser.ShowFirstLogin = settings.Env.IsFirstLoad && newUser.Permissions.Admin
	if userInfo.LoginMethod == "password" {
		if userInfo.Password == "" {
			return fmt.Errorf("password is required to create a password login user")
		}
	} else {
		hashpass, err := utils.HashPwd(userInfo.Username)
		if err != nil {
			return err
		}
		userInfo.Password = hashpass
	}
	// must have username
	if userInfo.Username == "" {
		return fmt.Errorf("username is required to create a user")
	}
	settings.ApplyUserDefaults(newUser)
	newUser.Permissions = permissions
	logger.Debugf("Creating user: %v %v", userInfo.Username, userInfo.Scopes)
	// create new home directories
	err := userStore.Save(newUser, true, false)
	if err != nil {
		return err
	}
	return nil
}
