package vault

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/coldforge/vault/internal/database"
	"github.com/coldforge/vault/internal/models"
	"github.com/google/uuid"
)

// SharingService handles folder sharing and team operations
type SharingService struct {
	db *database.DB
}

// NewSharingService creates a new sharing service
func NewSharingService(db *database.DB) *SharingService {
	return &SharingService{db: db}
}

// --- Team Operations ---

// CreateTeam creates a new team
func (s *SharingService) CreateTeam(ownerID uuid.UUID, req *models.CreateTeamRequest) (*models.Team, error) {
	team := &models.Team{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     ownerID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create team
	_, err = tx.Exec(`
		INSERT INTO teams (id, name, description, owner_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, team.ID, team.Name, team.Description, team.OwnerID, team.CreatedAt, team.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	// Add owner as team member
	_, err = tx.Exec(`
		INSERT INTO team_members (id, team_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, 'owner', $4)
	`, uuid.New(), team.ID, ownerID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to add owner to team: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return team, nil
}

// GetTeam retrieves a team by ID
func (s *SharingService) GetTeam(teamID, userID uuid.UUID) (*models.Team, error) {
	// Verify user is a member
	var memberCount int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM team_members WHERE team_id = $1 AND user_id = $2
	`, teamID, userID).Scan(&memberCount)
	if err != nil {
		return nil, fmt.Errorf("failed to verify membership: %w", err)
	}
	if memberCount == 0 {
		return nil, fmt.Errorf("team not found or access denied")
	}

	var team models.Team
	err = s.db.QueryRow(`
		SELECT id, name, description, owner_id, created_at, updated_at
		FROM teams WHERE id = $1
	`, teamID).Scan(
		&team.ID, &team.Name, &team.Description,
		&team.OwnerID, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("team not found")
		}
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	// Load members
	members, err := s.GetTeamMembers(teamID)
	if err != nil {
		return nil, err
	}
	team.Members = members

	return &team, nil
}

// ListUserTeams returns all teams a user belongs to
func (s *SharingService) ListUserTeams(userID uuid.UUID) ([]models.Team, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.name, t.description, t.owner_id, t.created_at, t.updated_at
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var team models.Team
		err := rows.Scan(
			&team.ID, &team.Name, &team.Description,
			&team.OwnerID, &team.CreatedAt, &team.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		teams = append(teams, team)
	}

	return teams, nil
}

// GetTeamMembers returns all members of a team
func (s *SharingService) GetTeamMembers(teamID uuid.UUID) ([]models.TeamMember, error) {
	rows, err := s.db.Query(`
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.joined_at,
		       u.email
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = $1
		ORDER BY tm.role, tm.joined_at
	`, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	defer rows.Close()

	var members []models.TeamMember
	for rows.Next() {
		var member models.TeamMember
		err := rows.Scan(
			&member.ID, &member.TeamID, &member.UserID,
			&member.Role, &member.JoinedAt, &member.Email,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, member)
	}

	return members, nil
}

// InviteToTeam creates an invitation to join a team
func (s *SharingService) InviteToTeam(teamID, inviterID uuid.UUID, req *models.InviteToTeamRequest) (*models.TeamInvitation, error) {
	// Verify inviter has permission
	var inviterRole string
	err := s.db.QueryRow(`
		SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2
	`, teamID, inviterID).Scan(&inviterRole)
	if err != nil {
		return nil, fmt.Errorf("not a team member")
	}
	if inviterRole != "owner" && inviterRole != "admin" {
		return nil, fmt.Errorf("insufficient permissions to invite")
	}

	if req.Email == nil && req.Pubkey == nil {
		return nil, fmt.Errorf("email or pubkey required")
	}

	invitation := &models.TeamInvitation{
		ID:            uuid.New(),
		TeamID:        teamID,
		InvitedBy:     inviterID,
		InvitedEmail:  req.Email,
		InvitedPubkey: req.Pubkey,
		Role:          req.Role,
		Status:        "pending",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().AddDate(0, 0, 7), // 7 days
	}

	_, err = s.db.Exec(`
		INSERT INTO team_invitations (id, team_id, invited_by, invited_email, invited_pubkey, role, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, invitation.ID, invitation.TeamID, invitation.InvitedBy,
		invitation.InvitedEmail, invitation.InvitedPubkey,
		invitation.Role, invitation.Status, invitation.CreatedAt, invitation.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	return invitation, nil
}

// AcceptTeamInvitation accepts an invitation and adds the user to the team
func (s *SharingService) AcceptTeamInvitation(invitationID, userID uuid.UUID) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get invitation
	var inv models.TeamInvitation
	err = tx.QueryRow(`
		SELECT id, team_id, role, status, expires_at
		FROM team_invitations WHERE id = $1
	`, invitationID).Scan(&inv.ID, &inv.TeamID, &inv.Role, &inv.Status, &inv.ExpiresAt)
	if err != nil {
		return fmt.Errorf("invitation not found")
	}

	if inv.Status != "pending" {
		return fmt.Errorf("invitation already %s", inv.Status)
	}
	if time.Now().After(inv.ExpiresAt) {
		return fmt.Errorf("invitation expired")
	}

	// Add user to team
	_, err = tx.Exec(`
		INSERT INTO team_members (id, team_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (team_id, user_id) DO NOTHING
	`, uuid.New(), inv.TeamID, userID, inv.Role, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	// Update invitation status
	_, err = tx.Exec(`
		UPDATE team_invitations SET status = 'accepted', accepted_at = $1 WHERE id = $2
	`, time.Now(), invitationID)
	if err != nil {
		return fmt.Errorf("failed to update invitation: %w", err)
	}

	return tx.Commit()
}

// --- Folder Sharing Operations ---

// ShareFolder shares a folder with a team or user
func (s *SharingService) ShareFolder(ownerID uuid.UUID, req *models.ShareFolderRequest) (*models.SharedFolder, error) {
	// Verify folder ownership
	var folderOwner uuid.UUID
	err := s.db.QueryRow(`SELECT user_id FROM vault_folders WHERE id = $1`, req.FolderID).Scan(&folderOwner)
	if err != nil {
		return nil, fmt.Errorf("folder not found")
	}
	if folderOwner != ownerID {
		return nil, fmt.Errorf("not the folder owner")
	}

	if req.TeamID == nil && req.UserID == nil {
		return nil, fmt.Errorf("team_id or user_id required")
	}

	shared := &models.SharedFolder{
		ID:              uuid.New(),
		FolderID:        req.FolderID,
		TeamID:          req.TeamID,
		SharedBy:        ownerID,
		SharedWith:      req.UserID,
		PermissionLevel: req.PermissionLevel,
		CreatedAt:       time.Now(),
		ExpiresAt:       req.ExpiresAt,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create shared folder record
	_, err = tx.Exec(`
		INSERT INTO shared_folders (id, folder_id, team_id, shared_by, shared_with, permission_level, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, shared.ID, shared.FolderID, shared.TeamID, shared.SharedBy,
		shared.SharedWith, shared.PermissionLevel, shared.CreatedAt, shared.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to share folder: %w", err)
	}

	// Store encrypted folder key for the recipient(s)
	if req.UserID != nil {
		_, err = tx.Exec(`
			INSERT INTO shared_folder_keys (id, folder_id, user_id, encrypted_folder_key, key_version, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 1, $5, $5)
			ON CONFLICT (folder_id, user_id, key_version) DO UPDATE SET
				encrypted_folder_key = EXCLUDED.encrypted_folder_key,
				updated_at = EXCLUDED.updated_at
		`, uuid.New(), req.FolderID, *req.UserID, req.EncryptedFolderKey, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to store folder key: %w", err)
		}
	}

	// Mark folder as shared
	_, err = tx.Exec(`UPDATE vault_folders SET is_shared = true WHERE id = $1`, req.FolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark folder as shared: %w", err)
	}

	return shared, tx.Commit()
}

// GetSharedFolders returns folders shared with a user
func (s *SharingService) GetSharedFolders(userID uuid.UUID) ([]models.SharedFolder, error) {
	rows, err := s.db.Query(`
		SELECT sf.id, sf.folder_id, sf.team_id, sf.shared_by, sf.shared_with,
		       sf.permission_level, sf.created_at, sf.expires_at,
		       vf.name, u.email
		FROM shared_folders sf
		JOIN vault_folders vf ON sf.folder_id = vf.id
		JOIN users u ON sf.shared_by = u.id
		WHERE sf.shared_with = $1
		   OR sf.team_id IN (SELECT team_id FROM team_members WHERE user_id = $1)
		ORDER BY sf.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared folders: %w", err)
	}
	defer rows.Close()

	var folders []models.SharedFolder
	for rows.Next() {
		var sf models.SharedFolder
		err := rows.Scan(
			&sf.ID, &sf.FolderID, &sf.TeamID, &sf.SharedBy, &sf.SharedWith,
			&sf.PermissionLevel, &sf.CreatedAt, &sf.ExpiresAt,
			&sf.FolderName, &sf.SharedByName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan shared folder: %w", err)
		}
		folders = append(folders, sf)
	}

	return folders, nil
}

// GetFolderKey retrieves the encrypted folder key for a user
func (s *SharingService) GetFolderKey(folderID, userID uuid.UUID) (*models.SharedFolderKey, error) {
	var key models.SharedFolderKey
	err := s.db.QueryRow(`
		SELECT id, folder_id, user_id, encrypted_folder_key, key_version, created_at, updated_at
		FROM shared_folder_keys
		WHERE folder_id = $1 AND user_id = $2
		ORDER BY key_version DESC
		LIMIT 1
	`, folderID, userID).Scan(
		&key.ID, &key.FolderID, &key.UserID,
		&key.EncryptedFolderKey, &key.KeyVersion,
		&key.CreatedAt, &key.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("folder key not found")
		}
		return nil, fmt.Errorf("failed to get folder key: %w", err)
	}

	return &key, nil
}

// RevokeShare removes a folder share
func (s *SharingService) RevokeShare(sharedFolderID, ownerID uuid.UUID) error {
	// Verify ownership through the folder
	var folderOwner uuid.UUID
	err := s.db.QueryRow(`
		SELECT vf.user_id FROM shared_folders sf
		JOIN vault_folders vf ON sf.folder_id = vf.id
		WHERE sf.id = $1
	`, sharedFolderID).Scan(&folderOwner)
	if err != nil {
		return fmt.Errorf("shared folder not found")
	}
	if folderOwner != ownerID {
		return fmt.Errorf("not authorized to revoke this share")
	}

	_, err = s.db.Exec(`DELETE FROM shared_folders WHERE id = $1`, sharedFolderID)
	if err != nil {
		return fmt.Errorf("failed to revoke share: %w", err)
	}

	return nil
}

// CheckFolderAccess verifies if a user has access to a folder
func (s *SharingService) CheckFolderAccess(folderID, userID uuid.UUID, requiredPermission string) (bool, error) {
	// Check if owner
	var ownerID uuid.UUID
	err := s.db.QueryRow(`SELECT user_id FROM vault_folders WHERE id = $1`, folderID).Scan(&ownerID)
	if err != nil {
		return false, nil
	}
	if ownerID == userID {
		return true, nil // Owner has full access
	}

	// Check shared access
	permissionLevels := map[string]int{"view": 1, "edit": 2, "admin": 3}
	requiredLevel := permissionLevels[requiredPermission]

	var actualPermission string
	err = s.db.QueryRow(`
		SELECT permission_level FROM shared_folders
		WHERE folder_id = $1
		AND (shared_with = $2 OR team_id IN (SELECT team_id FROM team_members WHERE user_id = $2))
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY
			CASE permission_level
				WHEN 'admin' THEN 3
				WHEN 'edit' THEN 2
				ELSE 1
			END DESC
		LIMIT 1
	`, folderID, userID).Scan(&actualPermission)

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return permissionLevels[actualPermission] >= requiredLevel, nil
}
