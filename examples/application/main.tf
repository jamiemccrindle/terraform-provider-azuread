resource "random_password" "widgets_service" {
  length  = 32
  special = true
}

resource "azuread_application" "widgets_service" {
  name = "widgets-service"
  type = "webapp/api"

  identifier_uris = ["api://widgets-service"]

  oauth2_permissions {
    admin_consent_description  = "Access Widgets Service as a user"
    admin_consent_display_name = "Access Widgets Service as a user"
    is_enabled                 = true
    type                       = "User"
    user_consent_description   = "Access Widgets Service as a user"
    user_consent_display_name  = "Access Widgets Service as a user"
    value                      = "access_as_user"
  }
}

resource "azuread_application_password" "widgets_service" {
  application_object_id = azuread_application.widgets_service.object_id
  value                 = random_password.widgets_service.result
  end_date_relative     = "17520h" # 2 years
}

resource "azuread_application" "widgets_app" {
  name = "widgets-app"
  type = "webapp/api"

  logout_url = "https://widgets.example.net/logout"
  reply_urls = [
    "https://widgets.example.net/",
    "https://widgets.example.net/login",
  ]

  oauth2_allow_implicit_flow = true

  required_resource_access {
    # Microsoft Graph
    resource_app_id = "00000003-0000-0000-c000-000000000000"

    resource_access {
      # User.Read
      id = "e1fe6dd8-ba31-4d61-89e7-88639da4683d"
      type = "Scope"
    }
  }

  required_resource_access {
    resource_app_id = azuread_application.widgets_service.application_id

    dynamic resource_access {
      for_each = azuread_application.widgets_service.oauth2_permissions
      iterator = scope

      content {
        id   = scope.value.id
        type = "Scope"
      }
    }
  }
}
