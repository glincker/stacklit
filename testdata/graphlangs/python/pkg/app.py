import os
from .base import Base, helper


class App(Base):
    repo: Base

    def run(self, b: Base):
        """Run the app."""
        helper(1)
        self.greet()
        b.greet()
        os.getcwd()
