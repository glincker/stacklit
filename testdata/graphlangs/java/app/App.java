package app;

import com.x.Base;

public class App extends Base {
    private Base repo;

    public void run(Base b) {
        greet();
        b.greet();
        Base.helper(1);
    }
}
