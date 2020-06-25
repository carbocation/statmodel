package statmodel

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strings"

	"gonum.org/v1/gonum/mat"
)

// Dtype is a type alias that is used to define the datatype of all data
// passed to the statistical models.  It should be set to float64 or float32.
type Dtype = float64

// Dataset defines a way to pass data to a statistical model.  A Dataset consists
// of one or more variables, each of which may be the response, a predictor, or some
// other variable in a statistical model (e.g. a weight or stratifying variable).
// Data()[k] is the k^th variable in the dataset, and the name of this variable is
// Varnames()[k].  Yname() and Xnames() return variables that are the dependent
// variable and independent variables in a regression model, respectively.
type Dataset interface {

	// Data returns all variables in the dataset, stored column-wise.
	Data() [][]Dtype

	// Varnames returns the names of the variables in the regression model,
	// in the same order as the data are returned by Data.
	Varnames() []string

	// Yname returns the name of the dependent variable in a regression model.
	Yname() string

	// Xnames returns the names of the independent variables in a regression model.
	Xnames() []string
}

type Columnser interface {
	Names() []string
	Data() [][]Dtype
}

func FromColumns(c Columnser, yname string, xnames []string) Dataset {

	na := c.Names()
	da := c.Data()

	return &basicData{
		data:     da,
		yname:    yname,
		varnames: na,
		xnames:   xnames,
	}
}

// basicData is a simple default implementation of the Dataset interface.
type basicData struct {
	data     [][]Dtype
	yname    string
	varnames []string
	xnames   []string
}

// NewDataset returns a dataset containing the given data columns.  varnames contains the names
// of the variables in the same order as the appear in data.  yname and xnames are names
// of the dependent and independent variables, respectively.
func NewDataset(data [][]Dtype, varnames []string, yname string, xnames []string) Dataset {

	if len(data) != len(varnames) {
		msg := fmt.Sprintf("len(data)=%d and len(varnames)=%d are not compatible\n", len(data), len(varnames))
		panic(msg)
	}

	return &basicData{
		data:     data,
		varnames: varnames,
		yname:    yname,
		xnames:   xnames,
	}
}

func (bd *basicData) Data() [][]Dtype {
	return bd.data
}

func (bd *basicData) Yname() string {
	return bd.yname
}

func (bd *basicData) Varnames() []string {
	return bd.varnames
}

func (bd *basicData) Xnames() []string {
	return bd.xnames
}

// HessType indicates the type of a Hessian matrix for a log-likelihood.
type HessType int

// ObsHess (observed Hessian) and ExpHess (expected Hessian) are the two type of log-likelihood
// Hessian matrices
const (
	ObsHess HessType = iota
	ExpHess
)

// Parameter is the parameter of a model.
type Parameter interface {

	// Get the coefficients of the covariates in the linear
	// predictor.  The returned value should be a reference so
	// that changes to it lead to corresponding changes in the
	// parameter itself.
	GetCoeff() []float64

	// Set the coefficients of the covariates in the linear
	// predictor.
	SetCoeff([]float64)

	// Clone creates a deep copy of the Parameter struct.
	Clone() Parameter
}

// RegFitter is a regression model that can be fit to data.
type RegFitter interface {

	// Number of parameters in the model.
	NumParams() int

	// Number of observations in the data set
	NumObs() int

	// Positions of the covariates
	Xpos() []int

	Dataset() [][]Dtype

	// The log-likelihood function
	LogLike(Parameter, bool) float64

	// The score vector
	Score(Parameter, []float64)

	// The Hessian matrix
	Hessian(Parameter, HessType, []float64)
}

// BaseResultser is a fitted model that can produce results (parameter estimates, etc.).
type BaseResultser interface {
	Model() RegFitter
	Names() []string
	LogLike() float64
	Params() []float64
	VCov() []float64
	StdErr() []float64
	ZScores() []float64
	PValues() []float64
}

// BaseResults contains the results after fitting a model to data.
type BaseResults struct {
	model   RegFitter
	loglike float64
	params  []float64
	xnames  []string
	vcov    []float64
	stderr  []float64
	zscores []float64
	pvalues []float64
}

// NewBaseResults returns a BaseResults corresponding to the given fitted model.
func NewBaseResults(model RegFitter, loglike float64, params []float64, xnames []string, vcov []float64) BaseResults {
	return BaseResults{
		model:   model,
		loglike: loglike,
		params:  params,
		xnames:  xnames,
		vcov:    vcov,
	}
}

// Model produces the model value used to produce the results.
func (rslt *BaseResults) Model() RegFitter {
	return rslt.model
}

// FittedValues returns the fitted linear predictor for a regression
// model.  If da is nil, the fitted values are based on the data used
// to fit the model.  Otherwise, the provided data stream is used to
// produce the fitted values, so it must have the same columns as the
// training data.
func (rslt *BaseResults) FittedValues(da [][]Dtype) []float64 {

	xpos := rslt.model.Xpos()

	if da == nil {
		// Use training data to get the fitted values
		da = rslt.model.Dataset()
	}

	if len(da) != len(rslt.model.Dataset()) {
		msg := fmt.Sprintf("Data has incorrect number of columns, %d != %d\n",
			len(da), len(rslt.model.Dataset()))
		panic(msg)
	}

	fv := make([]float64, rslt.model.NumObs())
	for k, j := range xpos {
		z := da[j]
		for i := range z {
			fv[i] += rslt.params[k] * float64(z[i])
		}
	}

	return fv
}

// Names returns the covariate names for the variables in the model.
func (rslt *BaseResults) Names() []string {
	return rslt.xnames
}

// Params returns the point estimates for the parameters in the model.
func (rslt *BaseResults) Params() []float64 {
	return rslt.params
}

// VCov returns the sampling variance/covariance model for the parameters in the model.
// The matrix is vetorized to one dimension.
func (rslt *BaseResults) VCov() []float64 {
	return rslt.vcov
}

// LogLike returns the log-likelihood or objective function value for the fitted model.
func (rslt *BaseResults) LogLike() float64 {
	return rslt.loglike
}

// StdErr returns the standard errors for the parameters in the model.
func (rslt *BaseResults) StdErr() []float64 {

	// No vcov, no standard error
	if rslt.vcov == nil {
		return nil
	}

	p := rslt.model.NumParams()
	if rslt.stderr == nil {
		rslt.stderr = make([]float64, p)
	} else {
		return rslt.stderr
	}

	for i := range rslt.stderr {
		rslt.stderr[i] = math.Sqrt(rslt.vcov[i*p+i])
	}

	return rslt.stderr
}

// ZScores returns the Z-scores (the parameter estimates divided by the standard errors).
func (rslt *BaseResults) ZScores() []float64 {

	// No vcov, no z-scores
	if rslt.vcov == nil {
		return nil
	}

	p := rslt.model.NumParams()
	if rslt.zscores == nil {
		rslt.zscores = make([]float64, p)
	} else {
		return rslt.zscores
	}

	std := rslt.StdErr()
	for i := range std {
		rslt.zscores[i] = rslt.params[i] / std[i]
	}

	return rslt.zscores
}

func normcdf(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt(2))
}

// PValues returns the p-values for the null hypothesis that each parameter's population
// value is equal to zero.
func (rslt *BaseResults) PValues() []float64 {

	// No vcov, no p-values
	if rslt.vcov == nil {
		return nil
	}

	p := rslt.model.NumParams()
	if rslt.pvalues == nil {
		rslt.pvalues = make([]float64, p)
	} else {
		return rslt.pvalues
	}

	for i, z := range rslt.zscores {
		rslt.pvalues[i] = 2 * normcdf(-math.Abs(z))
	}

	return rslt.pvalues
}

// GetVcov returns the sampling variance/covariance matrix for the parameter estimates.
func GetVcov(model RegFitter, params Parameter) ([]float64, error) {
	nvar := model.NumParams()
	n2 := nvar * nvar
	hess := make([]float64, n2)
	model.Hessian(params, ExpHess, hess)
	hmat := mat.NewDense(nvar, nvar, hess)
	hessi := make([]float64, n2)
	himat := mat.NewDense(nvar, nvar, hessi)
	err := himat.Inverse(hmat)
	if err != nil {
		os.Stderr.Write([]byte("Can't invert Hessian\n"))
		return nil, err
	}
	himat.Scale(-1, himat)

	return hessi, nil
}

// SummaryTable holds the summary values for a fitted model.
type SummaryTable struct {

	// Title
	Title string

	// Column names
	ColNames []string

	// Formatters for the column values
	ColFmt []Fmter

	// Cols[j] is the j^th column.  It's concrete type should
	// be an array, e.g. of numbers or strings.
	Cols []interface{}

	// Values at the top of the summary
	Top []string

	// Messages displayed below the table
	Msg []string

	// Total width of the table
	tw int
}

// Draw a line constructed of the given character filling the width of
// the table.
func (s *SummaryTable) line(c string) string {
	return strings.Repeat(c, s.tw) + "\n"
}

// cleanTop ensures that all fields in the top part of the table have
// the same width.
func (s *SummaryTable) cleanTop() {

	w := len(s.Top[0])
	for _, x := range s.Top {
		if len(x) > w {
			w = len(x)
		}
	}

	for i, x := range s.Top {
		if len(x) < w {
			s.Top[i] = x + strings.Repeat(" ", w-len(x))
		}
	}
}

// Construct the upper part of the table, which contains summary
// values for the model.
func (s *SummaryTable) top(gap int) string {

	w := []int{0, 0}

	for j, x := range s.Top {
		if len(x) > w[j%2] {
			w[j%2] = len(x)
		}
	}

	var b bytes.Buffer

	for j, x := range s.Top {
		c := fmt.Sprintf("%%-%ds", w[j%2])
		b.Write([]byte(fmt.Sprintf(c, x)))
		if j%2 == 1 {
			b.Write([]byte("\n"))
		} else {
			b.Write([]byte(strings.Repeat(" ", gap)))
		}
	}

	if len(s.Top)%2 == 1 {
		b.Write([]byte("\n"))
	}

	return b.String()
}

// Fmter formats the elements of an array of values.
type Fmter func(interface{}, string) []string

// String returns the table as a string.
func (s *SummaryTable) String() string {

	s.cleanTop()

	var tab [][]string
	var wx []int
	for j, c := range s.Cols {
		u := s.ColFmt[j](c, s.ColNames[j])
		tab = append(tab, u)
		if len(u[0]) > len(s.ColNames[j]) {
			wx = append(wx, len(u[0]))
		} else {
			wx = append(wx, len(s.ColNames[j]))
		}
	}

	gap := 10

	// Get the total width of the table
	s.tw = 0
	for _, w := range wx {
		s.tw += w
	}
	if s.tw < len(s.Title) {
		s.tw = len(s.Title)
	}
	if s.tw < gap+2*len(s.Top[0]) {
		s.tw = gap + 2*len(s.Top[0])
	}

	var buf bytes.Buffer

	// Center the title
	k := len(s.Title)
	kr := (s.tw - k) / 2
	if kr < 0 {
		kr = 0
	}
	buf.Write([]byte(strings.Repeat(" ", kr)))
	buf.Write([]byte(s.Title))
	buf.Write([]byte("\n"))

	buf.Write([]byte(s.line("=")))
	buf.Write([]byte(s.top(gap)))
	buf.Write([]byte(s.line("-")))

	for j, c := range s.ColNames {
		f := fmt.Sprintf("%%%ds", wx[j])
		buf.Write([]byte(fmt.Sprintf(f, c)))
	}
	buf.Write([]byte("\n"))
	buf.Write([]byte(s.line("-")))

	for i := 0; i < len(tab[0]); i++ {
		for j := 0; j < len(tab); j++ {
			f := fmt.Sprintf("%%%ds", wx[j])
			buf.Write([]byte(fmt.Sprintf(f, tab[j][i])))
		}
		buf.Write([]byte("\n"))
	}
	buf.Write([]byte(s.line("-")))

	if len(s.Msg) > 0 {
		for _, msg := range s.Msg {
			buf.Write([]byte(msg + "\n"))
		}
	}

	return buf.String()
}
