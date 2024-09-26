// build.rs

use std::process::{exit, Command};

fn main() {
    match Command::new("glslc")
        .args(&["shaders/shader.vert", "-o", "shaders/vert.spv"])
        .status()
    {
        Err(err) => {
            println!("{}", err);
            exit(1);
        }
        Ok(status) => {
            println!("{}", status);
        }
    }

    match Command::new("glslc")
        .args(&["shaders/shader.frag", "-o", "shaders/frag.spv"])
        .status()
    {
        Err(err) => {
            println!("{}", err);
            exit(1);
        }
        Ok(status) => {
            println!("{}", status);
        }
    }

    println!("cargo::rerun-if-changed=shaders/shader.vert");
    println!("cargo::rerun-if-changed=shaders/shader.frag");
}
