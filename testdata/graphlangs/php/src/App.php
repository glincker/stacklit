<?php
namespace App;

use Lib\Base;

class Main extends Base
{
    private Base $repo;

    public function run(Base $b)
    {
        $this->greet();
        $b->greet();
        $this->repo->greet();
        new Base();
    }
}
