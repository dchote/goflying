package ahrs

import (
	"math"
	"github.com/skelterjohn/go.matrix"
)

const (
	Pi = math.Pi
	G = 32.1740 / 1.687810  // G is the acceleration due to gravity, kt/s
	Small = 1e-9
	Big = 1e9
	Deg = Pi/180
	MMDecay = (1-1.0/50)           // Exponential decay constant for measurement variances
)

// State holds the complete information describing the state of the aircraft
// Aircraft frame is noninertial: 1 is to nose; 2 is to left wing; 3 is up
// Earth frame is inertial: 1 is east; 2 is north; 3 is up
// Sensor frame is fixed within aircraft frame, so noninertial, rotated
type State struct {
	U1, U2, U3    	float64             // Vector for airspeed, aircraft frame, kt
	Z1, Z2, Z3      float64             // Vector for rate of change of airspeed, aircraft frame, G
	E0, E1, E2, E3	float64             // Quaternion rotating earth frame to aircraft frame
	H1, H2, H3      float64             // Vector for gyro rates, earth frame, °/s
	N1, N2, N3    	float64             // Vector for earth's magnetic field, earth (inertial) frame, µT

	V1, V2, V3    	float64             // (Bias) Vector for windspeed, earth frame, kt
	C1, C2, C3      float64             // Bias vector for accelerometer, sensor frame, G
	F0, F1, F2, F3	float64             // (Bias) quaternion rotating sensor frame to aircraft frame
	D1, D2, D3      float64             // Bias vector for gyro rates, sensor frame, °/s
	L1, L2, L3      float64             // Bias vector for magnetometer direction, sensor frame, µT

	T             	float64             // Time when state last updated

	M             	*matrix.DenseMatrix // Covariance matrix of state uncertainty, same order as above vars:
	N               *matrix.DenseMatrix // Covariance matrix of state noise per unit time
	// U, Z, E, H, N,
	// V, C, F, D, L

	e11, e12, e13   float64             // cached quaternion fragment
	e21, e22, e23   float64             // cached quaternion fragment
	e31, e32, e33   float64             // cached quaternion fragment

	f11, f12, f13   float64             // cached quaternion fragment
	f21, f22, f23   float64             // cached quaternion fragment
	f31, f32, f33   float64             // cached quaternion fragment
}

// Measurement holds the measurements used for updating the Kalman filter: groundspeed, true airspeed, magnetometer
// Note: airspeed and magnetometer may not be available until appropriate sensors are working
type Measurement struct {                      // Order here also defines order in the matrices below
	UValid, WValid, SValid, MValid bool    // Do we have valid airspeed, GPS, accel/gyro, and magnetometer readings?
					       // U, W, A, B, M
	U1, U2, U3                     float64 // Vector of measured airspeed, kt, aircraft (accelerated) frame
	W1, W2, W3                     float64 // Vector of GPS speed in N/S, E/W and U/D directions, kt, latlong axes, earth (inertial) frame
	A1, A2, A3                     float64 // Vector holding accelerometer readings, G, aircraft (accelerated) frame
	B1, B2, B3                     float64 // Vector of gyro rates in roll, pitch, heading axes, °/s, aircraft (accelerated) frame
	M1, M2, M3                     float64 // Vector of magnetometer readings, µT, aircraft (accelerated) frame
	T                              float64 // Timestamp of GPS, airspeed and magnetometer readings
					       //TODO westphae: track separate measurement timestamps for Gyro/Accel, Magnetometer, GPS, Baro

	Accums                         [15]func(float64)(float64, float64, float64) // Accumulators to track means & variances of all variables

	M                              *matrix.DenseMatrix // Measurement noise covariance
}

func NewMeasurement() (*Measurement) {
	m := new(Measurement)

	m.M = matrix.Zeros(15, 15)

	m.Accums[0] = NewVarianceAccumulator(0, 1, MMDecay)
	m.Accums[1] = NewVarianceAccumulator(0, 1, MMDecay)
	m.Accums[2] = NewVarianceAccumulator(0, 1, MMDecay)
	m.Accums[3] = NewVarianceAccumulator(0, 0.2, MMDecay)
	m.Accums[4] = NewVarianceAccumulator(0, 0.2, MMDecay)
	m.Accums[5] = NewVarianceAccumulator(0, 0.2, MMDecay)
	m.Accums[6] = NewVarianceAccumulator(0, 0.3, MMDecay) // 0.006 typical from sensor
	m.Accums[7] = NewVarianceAccumulator(0, 0.3, MMDecay)
	m.Accums[8] = NewVarianceAccumulator(0, 0.3, MMDecay)
	m.Accums[9] = NewVarianceAccumulator(0, 1, MMDecay) // 0.03 typical from sensor
	m.Accums[10] = NewVarianceAccumulator(0, 1, MMDecay)
	m.Accums[11] = NewVarianceAccumulator(0, 1, MMDecay)
	m.Accums[12] = NewVarianceAccumulator(0, 80, MMDecay) //TODO westphae: get a feel for magnetometer noise
	m.Accums[13] = NewVarianceAccumulator(0, 80, MMDecay)
	m.Accums[14] = NewVarianceAccumulator(0, 80, MMDecay)

	return m
}

// normalize normalizes the E & F quaternions in State s
func (s *State) normalize() {
	ee := math.Sqrt(s.E0 * s.E0 + s.E1 * s.E1 + s.E2 * s.E2 + s.E3 * s.E3)
	s.E0 /= ee
	s.E1 /= ee
	s.E2 /= ee
	s.E3 /= ee

	ff := math.Sqrt(s.F0 * s.F0 + s.F1 * s.F1 + s.F2 * s.F2 + s.F3 * s.F3)
	s.F0 /= ff
	s.F1 /= ff
	s.F2 /= ff
	s.F3 /= ff

	s.calcRotationMatrices()
}

func (s *State) calcRotationMatrices() {
	// eij rotates between earth frame i component and aircraft frame j component
	s.e11 = (+s.E0 * s.E0 + s.E1 * s.E1 - s.E2 * s.E2 - s.E3 * s.E3)
	s.e12 = 2*(-s.E0 * s.E3 + s.E1 * s.E2)
	s.e13 = 2*(+s.E0 * s.E2 + s.E1 * s.E3)
	s.e21 = 2*(+s.E0 * s.E3 + s.E2 * s.E1)
	s.e22 = (+s.E0 * s.E0 - s.E1 * s.E1 + s.E2 * s.E2 - s.E3 * s.E3)
	s.e23 = 2*(-s.E0 * s.E1 + s.E2 * s.E3)
	s.e31 = 2*(-s.E0 * s.E2 + s.E3 * s.E1)
	s.e32 = 2*(+s.E0 * s.E1 + s.E3 * s.E2)
	s.e33 = (+s.E0 * s.E0 - s.E1 * s.E1 - s.E2 * s.E2 + s.E3 * s.E3)

	// fij rotates sensor frame i component into aircraft frame j component
	s.f11 = (+s.F0 * s.F0 + s.F1 * s.F1 - s.F2 * s.F2 - s.F3 * s.F3)
	s.f12 = 2*(-s.F0 * s.F3 + s.F1 * s.F2)
	s.f13 = 2*(+s.F0 * s.F2 + s.F1 * s.F3)
	s.f21 = 2*(+s.F0 * s.F3 + s.F2 * s.F1)
	s.f22 = (+s.F0 * s.F0 - s.F1 * s.F1 + s.F2 * s.F2 - s.F3 * s.F3)
	s.f23 = 2*(-s.F0 * s.F1 + s.F2 * s.F3)
	s.f31 = 2*(-s.F0 * s.F2 + s.F3 * s.F1)
	s.f32 = 2*(+s.F0 * s.F1 + s.F3 * s.F2)
	s.f33 = (+s.F0 * s.F0 - s.F1 * s.F1 - s.F2 * s.F2 + s.F3 * s.F3)
}

func (s *State) CalcRollPitchHeading() (roll float64, pitch float64, heading float64) {
	roll, pitch, heading = FromQuaternion(s.E0, s.E1, s.E2, s.E3)
	return
}

type AHRSProvider interface {
	Compute(*Measurement)
	Valid() (bool)
	CalcRollPitchHeading() (roll float64, pitch float64, heading float64)
	CalcRollPitchHeadingUncertainty() (droll float64, dpitch float64, dheading float64)
}