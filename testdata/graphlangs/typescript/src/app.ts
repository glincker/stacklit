import { Base, helper } from "./base";

export interface Runner {
  run(): void;
}

export class App extends Base implements Runner {
  private repo: Base;

  constructor(repo: Base) {
    super();
    this.repo = repo;
  }

  run(): void {
    helper(1);
    this.greet();
    this.repo.greet();
  }
}
