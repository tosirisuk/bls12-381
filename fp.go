package bls

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/sys/cpu"
)

var enforceNonBMI2 bool

var fpOne = *r1
var fpZero = fe{0, 0, 0, 0, 0, 0}

type fp struct {
	mul       func(c, a, b *fe)
	mulAssign func(a, b *fe)
}

func newFp() *fp {
	if cpu.X86.HasBMI2 && !enforceNonBMI2 {
		return &fp{
			mul:       montmul_bmi2,
			mulAssign: montmul_assign_bmi2,
		}
	}
	return &fp{
		mul:       montmul_nobmi2,
		mulAssign: montmul_assign_nobmi2,
	}
}

func (f *fp) fromBytes(in []byte) (*fe, error) {
	fe := &fe{}
	if len(in) != 48 {
		return nil, fmt.Errorf("input string should be equal 48 bytes")
	}
	fe.FromBytes(in)
	if !f.valid(fe) {
		return nil, fmt.Errorf("invalid input string")
	}
	f.mul(fe, fe, r2)
	return fe, nil
}

func (f *fp) fromUInt64(in uint64) (*fe, error) {
	fe := &fe{in}
	if in == 0 {
		return fe, nil
	}
	if !f.valid(fe) {
		return nil, fmt.Errorf("invalid input string")
	}
	f.mul(fe, fe, r2)
	return fe, nil
}

func (f *fp) fromBig(in *big.Int) (*fe, error) {
	fe := new(fe).SetBig(in)
	if !f.valid(fe) {
		return nil, fmt.Errorf("invalid input string")
	}
	f.mul(fe, fe, r2)
	return fe, nil
}

func (f *fp) fromString(in string) (*fe, error) {
	fe, err := new(fe).SetString(in)
	if err != nil {
		return nil, err
	}
	if !f.valid(fe) {
		return nil, fmt.Errorf("invalid input string")
	}
	f.mul(fe, fe, r2)
	return fe, nil
}

func (f *fp) toBytes(e *fe) []byte {
	e2 := new(fe)
	f.fromMont(e2, e)
	return e2.Bytes()
}

func (f *fp) toBig(e *fe) *big.Int {
	e2 := new(fe)
	f.fromMont(e2, e)
	return e2.Big()
}

func (f *fp) toString(e *fe) (s string) {
	e2 := new(fe)
	f.fromMont(e2, e)
	return e2.String()
}

func (f *fp) valid(fe *fe) bool {
	return fe.Cmp(&modulus) == -1
}

func (f *fp) zero() *fe {
	return &fe{}
}

func (f *fp) one() *fe {
	return new(fe).Set(r1)
}

func (f *fp) copy(dst *fe, src *fe) *fe {
	return dst.Set(src)
}

func (f *fp) rand(r io.Reader) (*fe, error) {
	fe := new(fe)
	bi, err := rand.Int(r, modulus.Big())
	if err != nil {
		return nil, err
	}
	return fe.SetBig(bi), nil
}

func (f *fp) equal(a, b *fe) bool {
	return a.Equals(b)
}

func (f *fp) isZero(a *fe) bool {
	return a.IsZero()
}

func (f *fp) isOne(a *fe) bool {
	return a.Equals(f.one())
}

func (f *fp) add(c, a, b *fe) {
	add6(c, a, b)
}

func (f *fp) addAssign(a, b *fe) {
	add_assign_6(a, b)
}

func (f *fp) ladd(c, a, b *fe) {
	ladd6(c, a, b)
}

func (f *fp) double(c, a *fe) {
	double6(c, a)
}

func (f *fp) doubleAssign(a *fe) {
	double_assign_6(a)
}

func (f *fp) ldouble(c, a *fe) {
	ldouble6(c, a)
}

func (f *fp) sub(c, a, b *fe) {
	sub6(c, a, b)
}

func (f *fp) subAssign(c, a *fe) {
	sub_assign_6(c, a)
}

func (f *fp) lsub(c, a, b *fe) {
	lsub6(c, a, b)
}

func (f *fp) neg(c, a *fe) {
	if a.IsZero() {
		c.Set(a)
	} else {
		neg(c, a)
	}
}

func (f *fp) toMont(c, a *fe) {
	f.mul(c, a, r2)
}

func (f *fp) fromMont(c, a *fe) {
	f.mul(c, a, &fe{1})
}

func (f *fp) square(c, a *fe) {
	f.mul(c, a, a)
}

func (f *fp) exp(c, a *fe, e *big.Int) {
	z := new(fe).Set(r1)
	for i := e.BitLen(); i >= 0; i-- {
		f.mul(z, z, z)
		if e.Bit(i) == 1 {
			f.mul(z, z, a)
		}
	}
	c.Set(z)
}

func (f *fp) inverse(inv, e *fe) {
	u := new(fe).Set(&modulus)
	v := new(fe).Set(e)
	s := &fe{1}
	r := &fe{0}
	var k int
	var z uint64
	var found = false
	// Phase 1
	for i := 0; i < 384*2; i++ {
		if v.IsZero() {
			found = true
			break
		}
		if u.IsEven() {
			u.div2(0)
			s.mul2()
		} else if v.IsEven() {
			v.div2(0)
			z += r.mul2()
		} else if u.Cmp(v) == 1 {
			lsub_assign_nc_6(u, v)
			u.div2(0)
			ladd_assign_6(r, s)
			s.mul2()
		} else {
			lsub_assign_nc_6(v, u)
			v.div2(0)
			ladd_assign_6(s, r)
			z += r.mul2()
		}
		k += 1
	}
	if found && k > 384 {
		if r.Cmp(&modulus) != -1 || z > 0 {
			lsub_assign_nc_6(r, &modulus)
		}
		u.Set(&modulus)
		lsub_assign_nc_6(u, r)
		// Phase 2
		for i := k; i < 384*2; i++ {
			double6(u, u)
		}
		inv.Set(u)
	} else {
		inv.Set(&fe{0})
	}
}

func (f *fp) sqrt(c, a *fe) (hasRoot bool) {
	var u, v fe
	f.copy(&u, a)
	f.exp(c, a, pPlus1Over4)
	f.square(&v, c)
	return f.equal(&u, &v)
}
