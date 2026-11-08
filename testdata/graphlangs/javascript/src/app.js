const { Base, helper } = require("./base");

class App extends Base {
  run() {
    helper(1);
    this.greet();
  }
}

const make = () => new App();
