<?php
namespace App\Billing;

use App\Models\User;
use Illuminate\Support\Collection;

interface Payable { public function pay(int $amount): bool; }

trait Loggable { public function log(string $m): void {} }

abstract class Invoice implements Payable
{
    use Loggable;
    public function total(float $tax): float { return 1.0; }
    private function hidden(): void {}
    public static function make(): self { return new self(); }
}

enum Status { case Open; case Paid; }

function helper(int $x): int { return $x; }
