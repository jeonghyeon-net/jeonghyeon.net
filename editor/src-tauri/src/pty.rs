use portable_pty::{native_pty_system, Child, CommandBuilder, MasterPty, PtySize};
use std::{
    collections::HashMap,
    io::{Read, Write},
    sync::Arc,
    thread,
};
use tauri::{async_runtime::Mutex as AsyncMutex, AppHandle, Emitter, State};

struct PtySession {
    master: Box<dyn MasterPty + Send>,
    writer: Box<dyn Write + Send>,
    child: Box<dyn Child + Send + Sync>,
    #[allow(dead_code)]
    child_pid: u32,
    #[allow(dead_code)]
    _reader_thread: thread::JoinHandle<()>,
}

pub struct PtyState {
    sessions: Arc<AsyncMutex<HashMap<u32, PtySession>>>,
    next_id: Arc<AsyncMutex<u32>>,
}

impl Default for PtyState {
    fn default() -> Self {
        Self {
            sessions: Arc::new(AsyncMutex::new(HashMap::new())),
            next_id: Arc::new(AsyncMutex::new(1)),
        }
    }
}

/// Find the safe boundary to emit, excluding incomplete escape sequences at the end.
/// Returns the byte index up to which it's safe to emit.
fn find_safe_emit_boundary(text: &str) -> usize {
    let bytes = text.as_bytes();
    let len = bytes.len();

    if len == 0 {
        return 0;
    }

    // Look for ESC (0x1b) in the last 64 bytes (escape sequences are typically short)
    let search_start = len.saturating_sub(64);

    // Find the last ESC character
    let mut last_esc = None;
    for i in (search_start..len).rev() {
        if bytes[i] == 0x1b {
            last_esc = Some(i);
            break;
        }
    }

    let esc_pos = match last_esc {
        Some(pos) => pos,
        None => return len, // No ESC found, safe to emit all
    };

    // Check if the escape sequence starting at esc_pos is complete
    let seq = &bytes[esc_pos..];

    if is_escape_sequence_complete(seq) {
        return len; // Complete sequence, safe to emit all
    }

    // Incomplete sequence, emit up to the ESC
    esc_pos
}

/// Check if an escape sequence is complete.
/// Returns true if the sequence is complete or not a recognized escape sequence.
fn is_escape_sequence_complete(seq: &[u8]) -> bool {
    if seq.is_empty() || seq[0] != 0x1b {
        return true;
    }

    if seq.len() == 1 {
        return false; // Just ESC, incomplete
    }

    match seq[1] {
        // CSI sequence: ESC [ ... (ends with 0x40-0x7E)
        b'[' => {
            if seq.len() == 2 {
                return false; // Just ESC [, incomplete
            }
            // Look for terminating character (0x40-0x7E: @A-Z[\]^_`a-z{|}~)
            for &b in &seq[2..] {
                if (0x40..=0x7E).contains(&b) {
                    return true; // Found terminator
                }
            }
            false // No terminator found
        }
        // OSC sequence: ESC ] ... (ends with BEL or ST)
        b']' => {
            for i in 2..seq.len() {
                if seq[i] == 0x07 {
                    return true; // BEL terminator
                }
                if seq[i] == 0x1b && i + 1 < seq.len() && seq[i + 1] == b'\\' {
                    return true; // ST terminator (ESC \)
                }
            }
            false
        }
        // DCS sequence: ESC P ... (ends with ST)
        b'P' => {
            for i in 2..seq.len() {
                if seq[i] == 0x1b && i + 1 < seq.len() && seq[i + 1] == b'\\' {
                    return true; // ST terminator
                }
            }
            false
        }
        // Single character sequences (complete with just one char after ESC)
        b'7' | b'8' | b'=' | b'>' | b'c' | b'D' | b'E' | b'H' | b'M' | b'N' | b'O'
        | b'Z' => true,
        // Unknown or other sequences - assume complete to avoid blocking
        _ => true,
    }
}

#[tauri::command]
pub async fn create_pty_session(
    app: AppHandle,
    state: State<'_, PtyState>,
    rows: u16,
    cols: u16,
    cwd: Option<String>,
) -> Result<u32, String> {
    let pty_system = native_pty_system();

    let pair = pty_system
        .openpty(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|e| format!("Failed to open pty: {}", e))?;

    let mut cmd = CommandBuilder::new_default_prog();
    if let Some(dir) = cwd {
        cmd.cwd(dir);
    }
    cmd.env("TERM", "xterm-256color");
    cmd.env("LANG", "en_US.UTF-8");
    cmd.env("LC_ALL", "en_US.UTF-8");

    let child = pair
        .slave
        .spawn_command(cmd)
        .map_err(|e| format!("Failed to spawn command: {}", e))?;

    drop(pair.slave);

    let writer = pair
        .master
        .take_writer()
        .map_err(|e| format!("Failed to get writer: {}", e))?;
    let mut reader = pair
        .master
        .try_clone_reader()
        .map_err(|e| format!("Failed to get reader: {}", e))?;

    let mut next_id = state.next_id.lock().await;
    let session_id = *next_id;
    *next_id += 1;

    let app_clone = app.clone();
    let reader_thread = thread::spawn(move || {
        let mut buf = [0u8; 8192];
        let mut pending: Vec<u8> = Vec::new();

        loop {
            match reader.read(&mut buf) {
                Ok(0) => {
                    if !pending.is_empty() {
                        let text = String::from_utf8_lossy(&pending).into_owned();
                        let _ = app_clone.emit(&format!("pty-output-{}", session_id), text);
                    }
                    let _ = app_clone.emit(&format!("pty-end-{}", session_id), ());
                    break;
                }
                Ok(n) => {
                    pending.extend_from_slice(&buf[..n]);

                    let valid_up_to = match std::str::from_utf8(&pending) {
                        Ok(_) => pending.len(),
                        Err(e) => e.valid_up_to(),
                    };

                    if valid_up_to > 0 {
                        let text =
                            unsafe { std::str::from_utf8_unchecked(&pending[..valid_up_to]) };

                        let emit_up_to = find_safe_emit_boundary(text);

                        if emit_up_to > 0 {
                            let to_emit = text[..emit_up_to].to_owned();
                            let _ = app_clone
                                .emit(&format!("pty-output-{}", session_id), to_emit);
                            pending.drain(..emit_up_to);
                        }
                    }

                    if pending.len() > 128 {
                        let text = String::from_utf8_lossy(&pending).into_owned();
                        let _ = app_clone.emit(&format!("pty-output-{}", session_id), text);
                        pending.clear();
                    }
                }
                Err(_) => {
                    let _ = app_clone.emit(&format!("pty-end-{}", session_id), ());
                    break;
                }
            }
        }
    });

    let child_pid = child.process_id().unwrap_or(0);
    let session = PtySession {
        master: pair.master,
        writer,
        child,
        child_pid,
        _reader_thread: reader_thread,
    };

    let mut sessions = state.sessions.lock().await;
    sessions.insert(session_id, session);

    Ok(session_id)
}

#[tauri::command]
pub async fn write_to_pty(
    state: State<'_, PtyState>,
    session_id: u32,
    data: String,
) -> Result<(), String> {
    let mut sessions = state.sessions.lock().await;
    if let Some(session) = sessions.get_mut(&session_id) {
        session
            .writer
            .write_all(data.as_bytes())
            .map_err(|e| format!("Write error: {}", e))?;
        session
            .writer
            .flush()
            .map_err(|e| format!("Flush error: {}", e))?;
        Ok(())
    } else {
        Err("Session not found".to_string())
    }
}

#[tauri::command]
pub async fn resize_pty(
    state: State<'_, PtyState>,
    session_id: u32,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    let sessions = state.sessions.lock().await;
    if let Some(session) = sessions.get(&session_id) {
        session
            .master
            .resize(PtySize {
                rows,
                cols,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| format!("Resize error: {}", e))?;
        Ok(())
    } else {
        Err("Session not found".to_string())
    }
}

#[tauri::command]
pub async fn close_pty_session(
    state: State<'_, PtyState>,
    session_id: u32,
) -> Result<(), String> {
    let session = {
        let mut sessions = state.sessions.lock().await;
        sessions.remove(&session_id)
    };

    if let Some(mut session) = session {
        let _ = session.child.kill();
        drop(session.master);
        drop(session.writer);
        Ok(())
    } else {
        Ok(())
    }
}
