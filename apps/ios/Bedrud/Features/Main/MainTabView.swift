import SwiftUI

enum AppTab: Hashable {
    case rooms
    case profile
    case settings
    case admin
    case join
}

struct MainTabView: View {
    @EnvironmentObject private var instanceManager: InstanceManager

    var body: some View {
        // BedrudApp shows this view only while authenticated, which requires an auth manager.
        if let authManager = instanceManager.authManager {
            MainTabContent(authManager: authManager)
        }
    }
}

/// Observes the auth manager itself, so the Admin tab follows the user the background profile refresh delivers.
private struct MainTabContent: View {
    @ObservedObject var authManager: AuthManager
    @State private var selectedTab: AppTab = .rooms
    @State private var showJoinSheet = false

    private var isAdmin: Bool {
        authManager.currentUser?.isAdmin == true
    }

    var body: some View {
        TabView(selection: tabSelection) {
            Tab("Rooms", systemImage: "rectangle.stack.fill", value: AppTab.rooms) {
                DashboardView()
            }

            Tab("Profile", systemImage: "person.fill", value: AppTab.profile) {
                ProfileView()
            }

            Tab("Settings", systemImage: "gearshape.fill", value: AppTab.settings) {
                SettingsView()
            }

            if isAdmin {
                Tab("Admin", systemImage: "shield.fill", value: AppTab.admin) {
                    AdminView()
                }
            }

            Tab("Join", systemImage: "link.badge.plus", value: AppTab.join, role: .search) {
                Text("")
            }
        }
        .tabViewStyle(.sidebarAdaptable)
        .sheet(isPresented: $showJoinSheet) {
            JoinByURLSheet()
        }
    }

    private var tabSelection: Binding<AppTab> {
        Binding {
            selectedTab
        } set: { newTab in
            if newTab == .join {
                showJoinSheet = true
            } else {
                selectedTab = newTab
            }
        }
    }
}
