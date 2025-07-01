use anyhow::Result;
use std::io::{self, Write};

pub struct ConversationApp {
    active: bool,
}

impl ConversationApp {
    pub fn new() -> Result<Self> {
        Ok(Self { active: true })
    }

    pub async fn run(&mut self) -> Result<()> {
        println!("🗄️  SQLTerm - Conversation Mode");
        println!("Type /help for available commands, or enter SQL queries directly.");
        println!();

        let mut input = String::new();
        
        loop {
            // Show prompt
            print!("sqlterm > ");
            io::stdout().flush().unwrap();
            
            // Read input
            input.clear();
            io::stdin().read_line(&mut input)?;
            let input = input.trim();
            
            if input.is_empty() {
                continue;
            }

            // Handle exit
            if input == "/quit" || input == "/exit" || input == "\\q" {
                println!("Goodbye! 👋");
                break;
            }

            // Handle help
            if input == "/help" {
                self.show_help();
                continue;
            }

            println!("You entered: {}", input);
        }

        Ok(())
    }

    fn show_help(&self) {
        println!("📋 Available Commands:");
        println!("  /help     - Show this help message");
        println!("  /quit     - Exit SQLTerm");
    }
}