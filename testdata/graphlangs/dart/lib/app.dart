import 'base.dart';

class App extends Base {
  final Base repo;
  App(this.repo);

  void run(Base b) {
    helper(1);
    greet();
    b.greet();
    repo.greet();
  }
}
