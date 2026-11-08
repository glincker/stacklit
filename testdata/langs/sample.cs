using System;
using System.Collections.Generic;
using static System.Math;

namespace Acme.Billing
{
    public interface IInvoice { decimal Total(); void Pay(int amount); string Name { get; } }

    public class Invoice : IInvoice
    {
        public string Id { get; set; }
        public decimal Total() { return 1m; }
        private void Hidden() {}
        public static Invoice Create(string id, int qty) { return null; }
    }

    public record Money(decimal Amount);
    public enum Status { Open, Paid }
    public struct Point { public int X; }
    internal class Secret {}
}
