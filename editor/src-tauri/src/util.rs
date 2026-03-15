pub fn extended_path() -> String {
    let current_path = std::env::var("PATH").unwrap_or_default();
    format!(
        "/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:{}",
        current_path
    )
}

/// Validates that a path is absolute and does not contain traversal components.
pub fn is_safe_path(path: &str) -> bool {
    !path.contains("..") && std::path::Path::new(path).is_absolute()
}
