// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

#[cfg(target_os = "windows")]
fn setup_console_encoding() {
    use std::process::Command;
    let _ = Command::new("chcp").arg("65001").output();
    unsafe {
        #[link(name = "kernel32")]
        extern "system" {
            fn SetConsoleOutputCP(wCodePageID: u32) -> i32;
            fn SetConsoleCP(wCodePageID: u32) -> i32;
        }
        SetConsoleOutputCP(65001); // UTF-8
        SetConsoleCP(65001);
    }
}

fn main() {
    #[cfg(target_os = "windows")]
    setup_console_encoding();

    desktop_lib::run()
}
