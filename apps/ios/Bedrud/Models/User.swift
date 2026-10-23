import Foundation

// MARK: - User

struct User: Codable, Identifiable, Equatable, Sendable {
    let id: String
    let email: String
    let name: String
    let avatarUrl: String?
    let accesses: [String]?
    let provider: String?

    /// The access level the server's `/admin` routes require.
    static let adminAccess = "superadmin"

    /// Whether the user can use the admin screens, read from the server's `accesses` list.
    var isAdmin: Bool { accesses?.contains(Self.adminAccess) == true }

    static func == (lhs: User, rhs: User) -> Bool {
        lhs.id == rhs.id
    }
}


