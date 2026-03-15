mod files;
mod git;
mod image;
mod preview;
mod pty;
mod util;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_dialog::init())
        .manage(pty::PtyState::default())
        .manage(files::WatcherState(std::sync::Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![
            pty::create_pty_session,
            pty::write_to_pty,
            pty::resize_pty,
            pty::close_pty_session,
            files::read_file,
            files::write_file,
            files::rename_path,
            files::delete_path,
            files::list_content_tree,
            files::watch_content_dir,
            git::get_repo_path,
            git::ensure_repo_cloned,
            git::check_hooks_configured,
            git::setup_hooks,
            git::ensure_transformer_built,
            image::optimize_image,
            preview::render_single_file,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
