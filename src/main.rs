use anyhow::Result;

use engine::Engine;

fn main() -> Result<()> {
    pretty_env_logger::init();
    
    let engine = Engine::new();
    match engine {
        Err(err) => println!("{}", err),
        Ok(e) => {
            e.run()?
        }
    }

    Ok(())
}
