using Lib;

namespace App
{
    public class AppMain : Base
    {
        private Base _repo;

        public void Run(Base b)
        {
            Greet();
            b.Greet();
            _repo.Greet();
            Base.Helper(1);
        }
    }
}
