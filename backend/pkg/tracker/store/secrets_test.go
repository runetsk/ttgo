package store

import (
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/secretbox"

	"github.com/stretchr/testify/require"
)

// foreignCiphertext encrypts plain under a key this store does not hold: what a lost or
// rotated secret.key, or a DB restored from another instance, leaves behind.
func foreignCiphertext(t *testing.T, plain string) string {
	t.Helper()
	other, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	enc, err := other.Encrypt(plain)
	require.NoError(t, err)
	return enc
}

func rawColumn(t *testing.T, s *Store, table, column, id string) string {
	t.Helper()
	var v string
	require.NoError(t, s.db.Raw("SELECT "+column+" FROM "+table+" WHERE id = ?", id).Scan(&v).Error)
	return v
}

func TestJiraToken_UndecryptableIsReportedNeverReturned(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpsertJiraConfig("https://x.atlassian.net", "a@b.c", "jira-secret-1234", true, "PROJ", "Bug")
	require.NoError(t, err)
	foreign := foreignCiphertext(t, "jira-secret-1234")
	require.NoError(t, s.db.Model(&models.JiraConfig{}).Where("id = ?", jiraConfigSingletonID).Update("api_token", foreign).Error)

	cfg, err := s.GetJiraConfig()
	require.NoError(t, err, "an undecryptable token must not fail the read")
	require.Equal(t, "", cfg.APIToken, "never the ciphertext")
	require.Equal(t, models.SecretStatusUndecryptable, cfg.APITokenStatus)
	require.ErrorIs(t, cfg.TokenError(), models.ErrSecretUndecryptable)
	require.EqualError(t, cfg.TokenError(), "Jira API token: the stored key can't be decrypted — re-enter it")
	resp := cfg.MaskedConfig()
	require.Equal(t, models.SecretStatusUndecryptable, resp.APITokenStatus)
	require.Equal(t, "", resp.APITokenMasked)

	// Editing every other field keeps the stored ciphertext byte-for-byte.
	_, err = s.UpsertJiraConfig("https://y.atlassian.net", "c@d.e", "", false, "OTHER", "Task")
	require.NoError(t, err)
	require.Equal(t, foreign, rawColumn(t, s, "jira_configs", "api_token", jiraConfigSingletonID))
	cfg, err = s.GetJiraConfig()
	require.NoError(t, err)
	require.Equal(t, "https://y.atlassian.net", cfg.BaseURL)
	require.False(t, cfg.Enabled)
	require.Equal(t, models.SecretStatusUndecryptable, cfg.APITokenStatus)

	// Clearing, then re-entering, recovers it.
	require.NoError(t, s.ClearJiraAPIToken())
	cfg, err = s.GetJiraConfig()
	require.NoError(t, err)
	require.Equal(t, models.SecretStatusMissing, cfg.MaskedConfig().APITokenStatus)
	require.NoError(t, cfg.TokenError())
	_, err = s.UpsertJiraConfig("https://y.atlassian.net", "c@d.e", "jira-new-5678", true, "OTHER", "Task")
	require.NoError(t, err)
	cfg, err = s.GetJiraConfig()
	require.NoError(t, err)
	require.Equal(t, "jira-new-5678", cfg.APIToken)
	require.Equal(t, models.SecretStatusOK, cfg.APITokenStatus)
	require.Equal(t, "****5678", cfg.MaskedConfig().APITokenMasked)
}

func TestConfluenceToken_KeepAndClear(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpsertConfluenceConfig("https://x.atlassian.net", "a@b.c", "conf-secret", true)
	require.NoError(t, err)
	foreign := foreignCiphertext(t, "conf-secret")
	require.NoError(t, s.db.Model(&models.ConfluenceConfig{}).Where("id = ?", confluenceConfigSingletonID).Update("api_token", foreign).Error)

	cfg, err := s.UpsertConfluenceConfig("https://z.atlassian.net", "a@b.c", "", false)
	require.NoError(t, err)
	require.Equal(t, "https://z.atlassian.net", cfg.BaseURL)
	require.Equal(t, models.SecretStatusUndecryptable, cfg.ToResponse().APITokenStatus)
	require.False(t, cfg.ToResponse().HasToken)
	require.Equal(t, foreign, rawColumn(t, s, "confluence_configs", "api_token", confluenceConfigSingletonID))

	require.NoError(t, s.ClearConfluenceAPIToken())
	require.Equal(t, "", rawColumn(t, s, "confluence_configs", "api_token", confluenceConfigSingletonID))
	cfg, err = s.GetConfluenceConfig()
	require.NoError(t, err)
	require.Equal(t, models.SecretStatusMissing, cfg.ToResponse().APITokenStatus)
}

func TestConfluenceToken_PlaintextIsUndecryptableUntilBackfilled(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpsertConfluenceConfig("https://x.atlassian.net", "a@b.c", "conf-secret", true)
	require.NoError(t, err)
	// Plaintext can only be here if the boot backfill failed (or a manual DB edit).
	require.NoError(t, s.db.Model(&models.ConfluenceConfig{}).Where("id = ?", confluenceConfigSingletonID).Update("api_token", "plain-conf-token").Error)

	cfg, err := s.GetConfluenceConfig()
	require.NoError(t, err)
	require.Equal(t, "", cfg.APIToken)
	require.EqualError(t, cfg.TokenError(), "Confluence API token: the stored key can't be decrypted — re-enter it")

	require.NoError(t, s.backfillEncryptSecrets())
	cfg, err = s.GetConfluenceConfig()
	require.NoError(t, err)
	require.Equal(t, "plain-conf-token", cfg.APIToken)
	require.Equal(t, models.SecretStatusOK, cfg.APITokenStatus)
}

func TestProviderKey_UndecryptableKeepsListAndAdminActionsWorking(t *testing.T) {
	s := newTestStore(t)
	good := &models.LLMProviderConfig{Label: "good", ProviderType: "openai", APIKey: "sk-good-1111", ModelName: "m", Enabled: true}
	bad := &models.LLMProviderConfig{Label: "bad", ProviderType: "openai", APIKey: "sk-bad-2222", ModelName: "m", Enabled: true}
	require.NoError(t, s.CreateProviderConfig(good))
	require.NoError(t, s.CreateProviderConfig(bad))
	require.Equal(t, models.SecretStatusOK, bad.APIKeyStatus, "create reports the status of what it saved")
	require.NoError(t, s.SetDefaultProviderConfig(bad.ID))
	foreign := foreignCiphertext(t, "sk-bad-2222")
	require.NoError(t, s.db.Model(&models.LLMProviderConfig{}).Where("id = ?", bad.ID).Update("api_key", foreign).Error)

	all, err := s.GetAllProviderConfigs()
	require.NoError(t, err, "one undecryptable key must not fail the list")
	require.Len(t, all, 2)
	byLabel := map[string]*models.LLMProviderConfig{}
	for _, c := range all {
		byLabel[c.Label] = c
	}
	require.Equal(t, "sk-good-1111", byLabel["good"].APIKey)
	require.Equal(t, models.SecretStatusOK, byLabel["good"].MaskedConfig().APIKeyStatus)
	require.Equal(t, "", byLabel["bad"].APIKey)
	require.Equal(t, models.SecretStatusUndecryptable, byLabel["bad"].MaskedConfig().APIKeyStatus)
	require.Equal(t, "", byLabel["bad"].MaskedConfig().APIKeyMasked)

	def, err := s.GetDefaultProviderConfig()
	require.NoError(t, err)
	require.EqualError(t, def.KeyError(), `LLM provider "bad" API key: the stored key can't be decrypted — re-enter it`)
	require.NoError(t, byLabel["good"].KeyError())

	// Non-secret edits (model, disable, default) work and leave the ciphertext untouched.
	upd, err := s.UpdateProviderConfig(bad.ID, map[string]interface{}{"model_name": "m2", "enabled": false}, "")
	require.NoError(t, err)
	require.Equal(t, "m2", upd.ModelName)
	require.False(t, upd.Enabled)
	require.Equal(t, models.SecretStatusUndecryptable, upd.APIKeyStatus)
	require.Equal(t, foreign, rawColumn(t, s, "llm_provider_configs", "api_key", bad.ID))
	require.NoError(t, s.SetDefaultProviderConfig(good.ID))

	// Clear, then re-enter.
	upd, err = s.UpdateProviderConfig(bad.ID, map[string]interface{}{"api_key": ""}, "")
	require.NoError(t, err)
	require.Equal(t, models.SecretStatusMissing, upd.APIKeyStatus)
	upd, err = s.UpdateProviderConfig(bad.ID, map[string]interface{}{}, "sk-new-3333")
	require.NoError(t, err)
	require.Equal(t, "sk-new-3333", upd.APIKey)
	require.Equal(t, models.SecretStatusOK, upd.APIKeyStatus)
	require.True(t, secretbox.IsEncrypted(rawColumn(t, s, "llm_provider_configs", "api_key", bad.ID)))

	require.NoError(t, s.DeleteProviderConfig(bad.ID))
}

func TestSecrets_EncryptionFailureRefusesTheSave(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpsertJiraConfig("https://x.atlassian.net", "a@b.c", "old-token", true, "PROJ", "Bug")
	require.NoError(t, err)
	p := &models.LLMProviderConfig{Label: "p", ProviderType: "openai", APIKey: "sk-old", ModelName: "m", Enabled: true}
	require.NoError(t, s.CreateProviderConfig(p))
	beforeJira := rawColumn(t, s, "jira_configs", "api_token", jiraConfigSingletonID)
	beforeKey := rawColumn(t, s, "llm_provider_configs", "api_key", p.ID)

	saved := s.box
	s.box = nil // an unavailable box: the strict helper must refuse, never store plaintext
	defer func() { s.box = saved }()

	_, err = s.UpsertJiraConfig("https://x.atlassian.net", "a@b.c", "new-token", true, "PROJ", "Bug")
	require.ErrorIs(t, err, models.ErrSecretNotSaved)
	require.ErrorIs(t, err, ErrEncryptionUnavailable)
	require.Equal(t, beforeJira, rawColumn(t, s, "jira_configs", "api_token", jiraConfigSingletonID), "a refused save changes nothing")

	_, err = s.UpsertConfluenceConfig("https://x.atlassian.net", "a@b.c", "new-token", true)
	require.ErrorIs(t, err, models.ErrSecretNotSaved)
	conf, err := s.getConfluenceConfigRaw()
	require.NoError(t, err)
	require.Nil(t, conf, "nothing was created")

	err = s.CreateProviderConfig(&models.LLMProviderConfig{Label: "q", ProviderType: "openai", APIKey: "sk-new", ModelName: "m", Enabled: true})
	require.ErrorIs(t, err, models.ErrSecretNotSaved)
	var n int64
	require.NoError(t, s.db.Model(&models.LLMProviderConfig{}).Count(&n).Error)
	require.Equal(t, int64(1), n)

	_, err = s.UpdateProviderConfig(p.ID, map[string]interface{}{"model_name": "m2"}, "sk-new")
	require.ErrorIs(t, err, models.ErrSecretNotSaved)
	require.Equal(t, beforeKey, rawColumn(t, s, "llm_provider_configs", "api_key", p.ID))
	require.Equal(t, "m", rawColumn(t, s, "llm_provider_configs", "model_name", p.ID), "the whole update is refused")
}

func TestSeedDemo_ProviderKeyIsStoredEncrypted(t *testing.T) {
	s := newTestStore(t)
	_, err := s.SeedDemoTx(false)
	require.NoError(t, err)
	all, err := s.GetAllProviderConfigs()
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for _, p := range all {
		require.Equal(t, models.SecretStatusOK, p.APIKeyStatus, p.Label)
		require.True(t, secretbox.IsEncrypted(rawColumn(t, s, "llm_provider_configs", "api_key", p.ID)), p.Label)
	}
}
