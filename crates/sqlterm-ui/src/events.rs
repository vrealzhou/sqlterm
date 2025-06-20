use crossterm::event::{self, Event as CrosstermEvent, KeyEvent, MouseEvent};
use std::time::{Duration, Instant};
use tokio::sync::mpsc;

#[derive(Clone, Debug)]
pub enum Event {
    Key(KeyEvent),
    Mouse(MouseEvent),
    Resize(u16, u16),
    Tick,
}

pub struct EventHandler {
    _tx: mpsc::UnboundedSender<Event>,
    rx: mpsc::UnboundedReceiver<Event>,
}

impl EventHandler {
    pub fn new(tick_rate: u64) -> Self {
        let tick_rate = Duration::from_millis(tick_rate);
        let (tx, rx) = mpsc::unbounded_channel();
        let _tx = tx.clone();

        tokio::spawn(async move {
            let mut last_tick = Instant::now();
            loop {
                let timeout = tick_rate
                    .checked_sub(last_tick.elapsed())
                    .unwrap_or_else(|| Duration::from_secs(0));

                match event::poll(timeout) {
                    Ok(true) => {
                        match event::read() {
                            Ok(CrosstermEvent::Key(e)) => {
                                if tx.send(Event::Key(e)).is_err() {
                                    break;
                                }
                            }
                            Ok(CrosstermEvent::Mouse(e)) => {
                                if tx.send(Event::Mouse(e)).is_err() {
                                    break;
                                }
                            }
                            Ok(CrosstermEvent::Resize(w, h)) => {
                                if tx.send(Event::Resize(w, h)).is_err() {
                                    break;
                                }
                            }
                            Ok(_) => {}
                            Err(_) => break, // Exit on read error
                        }
                    }
                    Ok(false) => {} // No event available
                    Err(_) => break, // Exit on poll error
                }

                if last_tick.elapsed() >= tick_rate {
                    if tx.send(Event::Tick).is_err() {
                        break;
                    }
                    last_tick = Instant::now();
                }
            }
        });

        Self { _tx, rx }
    }

    pub async fn next(&mut self) -> Result<Event, Box<dyn std::error::Error>> {
        self.rx.recv().await.ok_or_else(|| "Event channel closed".into())
    }
}
