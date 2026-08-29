// MOSAIC desktop shell (Tauri 2). This file is deliberately thin: its only
// job is to launch the Go engine binary as a sidecar process, capture the
// local port it prints on boot, and hand that port to the React frontend.
// No data processing, parsing or transformation logic lives here or in
// TypeScript — everything data-related happens in the Go engine over the
// local HTTP bridge (see backend/internal/bridge).
use std::sync::Mutex;
use tauri::{Manager, State};
use tauri_plugin_shell::ShellExt;
use tauri_plugin_shell::process::CommandEvent;

struct EnginePort(Mutex<Option<u16>>);

#[tauri::command]
fn get_engine_port(state: State<EnginePort>) -> Option<u16> {
    *state.0.lock().unwrap()
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .manage(EnginePort(Mutex::new(None)))
        .setup(|app| {
            let handle = app.handle().clone();

            // Spawn the Go engine sidecar (built separately as
            // `mosaic-engine` and referenced from tauri.conf.json's
            // bundle.externalBin — see README setup instructions).
            let shell = handle.shell();
            let (mut rx, _child) = shell
                .sidecar("mosaic-engine")
                .expect("failed to create mosaic-engine sidecar command")
                .spawn()
                .expect("failed to spawn mosaic-engine sidecar");

            let handle_for_events = handle.clone();
            tauri::async_runtime::spawn(async move {
                while let Some(event) = rx.recv().await {
                    if let CommandEvent::Stdout(line) = event {
                        let text = String::from_utf8_lossy(&line);
                        if let Some(rest) = text.trim().strip_prefix("MOSAIC_ENGINE_PORT=") {
                            if let Ok(port) = rest.parse::<u16>() {
                                let state: State<EnginePort> = handle_for_events.state();
                                *state.0.lock().unwrap() = Some(port);
                                let _ = handle_for_events.emit("engine-ready", port);
                            }
                        }
                    }
                }
            });

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![get_engine_port])
        .run(tauri::generate_context!())
        .expect("error while running the MOSAIC desktop shell");
}
