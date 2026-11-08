package app

import lib.Base
import lib.Util

class App(repo: Base) extends Base {
  def run(b: Base): Unit = {
    Util.helper(1)
    greet()
    b.greet()
    repo.greet()
  }
}
