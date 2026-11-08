mod base;

use crate::base::{helper, Base};

pub struct App {
    inner: Base,
}

impl App {
    pub fn run(&self) {
        helper();
        self.inner.greet();
    }
}
