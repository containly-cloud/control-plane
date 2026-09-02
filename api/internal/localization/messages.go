// Package localization owns public API copy. Internal logs deliberately do not
// use this package and remain in English for consistent operations.
package localization

import "strings"

type Locale string
type Key string

const (
	PortugueseBrazil Locale = "pt-BR"
	EnglishUS        Locale = "en-US"

	InitialSetupUnavailable    Key = "initial_setup_unavailable"
	InvalidSetupInput          Key = "invalid_setup_input"
	InitialSetupComplete       Key = "initial_setup_complete"
	WeakPassword               Key = "weak_password"
	AccountCreationFailed      Key = "account_creation_failed"
	AccountCreated             Key = "account_created"
	InvalidLoginInput          Key = "invalid_login_input"
	InvalidCredentials         Key = "invalid_credentials"
	Unauthenticated            Key = "unauthenticated"
	AuthenticationUnavailable  Key = "authentication_unavailable"
	AccessDenied               Key = "access_denied"
	SystemOverviewUnavailable  Key = "system_overview_unavailable"
	SettingsUnavailable        Key = "settings_unavailable"
	InvalidMonitoringSettings  Key = "invalid_monitoring_settings"
	AccountSessionsUnavailable Key = "account_sessions_unavailable"
	BackupsUnavailable         Key = "backups_unavailable"
	BackupCreationFailed       Key = "backup_creation_failed"
	BackupDeletionFailed       Key = "backup_deletion_failed"
	UsersUnavailable           Key = "users_unavailable"
	InvalidUserInput           Key = "invalid_user_input"
	UserCreationFailed         Key = "user_creation_failed"
	APIRouteNotFound           Key = "api_route_not_found"
)

var messages = map[Locale]map[Key]string{
	PortugueseBrazil: {
		InitialSetupUnavailable:    "Não foi possível carregar a configuração inicial.",
		InvalidSetupInput:          "Informe um nome de usuário e uma senha válidos.",
		InitialSetupComplete:       "A configuração inicial já foi concluída.",
		WeakPassword:               "A senha deve ter pelo menos {{min}} caracteres.",
		AccountCreationFailed:      "Não foi possível criar a conta de administrador. Verifique os dados informados.",
		AccountCreated:             "Conta de administrador criada.",
		InvalidLoginInput:          "Informe um nome de usuário e uma senha válidos.",
		InvalidCredentials:         "O nome de usuário ou a senha está incorreto.",
		Unauthenticated:            "Faça login para continuar.",
		AuthenticationUnavailable:  "Não foi possível concluir a autenticação.",
		AccessDenied:               "Você não tem permissão para realizar esta ação.",
		SystemOverviewUnavailable:  "Não foi possível carregar as informações do sistema.",
		SettingsUnavailable:        "Não foi possível carregar as configurações.",
		InvalidMonitoringSettings:  "As configurações de monitoramento são inválidas.",
		AccountSessionsUnavailable: "Não foi possível carregar as sessões conectadas.",
		BackupsUnavailable:         "Não foi possível carregar os backups.",
		BackupCreationFailed:       "Não foi possível criar o backup.",
		BackupDeletionFailed:       "Não foi possível excluir o backup.",
		UsersUnavailable:           "Não foi possível carregar os usuários do Control Plane.",
		InvalidUserInput:           "Informe os dados de usuário válidos.",
		UserCreationFailed:         "Não foi possível criar o usuário do Control Plane.",
		APIRouteNotFound:           "Rota de API não encontrada.",
	},
	EnglishUS: {
		InitialSetupUnavailable:    "Unable to load the initial setup.",
		InvalidSetupInput:          "Provide a valid username and password.",
		InitialSetupComplete:       "Initial setup has already been completed.",
		WeakPassword:               "Password must contain at least {{min}} characters.",
		AccountCreationFailed:      "Unable to create the administrator account. Check the information provided.",
		AccountCreated:             "Administrator account created.",
		InvalidLoginInput:          "Provide a valid username and password.",
		InvalidCredentials:         "The username or password is incorrect.",
		Unauthenticated:            "Sign in to continue.",
		AuthenticationUnavailable:  "Unable to complete authentication.",
		AccessDenied:               "You do not have permission to perform this action.",
		SystemOverviewUnavailable:  "Unable to load system information.",
		SettingsUnavailable:        "Unable to load settings.",
		InvalidMonitoringSettings:  "The monitoring settings are invalid.",
		AccountSessionsUnavailable: "Unable to load connected sessions.",
		BackupsUnavailable:         "Unable to load backups.",
		BackupCreationFailed:       "Unable to create the backup.",
		BackupDeletionFailed:       "Unable to delete the backup.",
		UsersUnavailable:           "Unable to load Control Plane users.",
		InvalidUserInput:           "Provide valid user details.",
		UserCreationFailed:         "Unable to create the Control Plane user.",
		APIRouteNotFound:           "API route not found.",
	},
}

// FromAcceptLanguage selects one of the compiled public API locales.
// Portuguese (Brazil) is the default for unsupported or omitted preferences.
func FromAcceptLanguage(header string) Locale {
	for _, item := range strings.Split(header, ",") {
		language := strings.ToLower(strings.TrimSpace(strings.SplitN(item, ";", 2)[0]))
		switch {
		case language == "en-us", strings.HasPrefix(language, "en-"):
			return EnglishUS
		case language == "pt-br", strings.HasPrefix(language, "pt-"):
			return PortugueseBrazil
		}
	}
	return PortugueseBrazil
}

func Message(locale Locale, key Key, parameters map[string]string) string {
	resolved := ""
	if translated, ok := messages[locale][key]; ok {
		resolved = translated
	} else {
		resolved = messages[PortugueseBrazil][key]
	}
	for name, value := range parameters {
		resolved = strings.ReplaceAll(resolved, "{{"+name+"}}", value)
	}
	return resolved
}
